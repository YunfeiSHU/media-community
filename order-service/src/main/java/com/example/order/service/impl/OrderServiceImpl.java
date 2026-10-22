package com.example.order.service.impl;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;
import com.example.order.common.IdGenerator;
import com.example.order.common.PageVO;
import com.example.order.config.OrderProperties;
import com.example.order.domain.entity.Order;
import com.example.order.domain.entity.OrderItem;
import com.example.order.domain.enums.OrderStatus;
import com.example.order.domain.enums.PaymentStatus;
import com.example.order.domain.enums.ProductStatus;
import com.example.order.domain.enums.SkuStatus;
import com.example.order.dto.request.CreateOrderRequest;
import com.example.order.dto.request.CreateOrderRequest.OrderItemRequest;
import com.example.order.dto.response.OrderVO;
import com.example.order.mapper.OrderItemMapper;
import com.example.order.mapper.OrderMapper;
import com.example.order.mapper.PaymentOrderMapper;
import com.example.order.service.OrderService;
import com.example.order.support.OrderLookup;
import com.example.order.support.ProductRpcClient;
import com.example.product.api.dto.ProductDTO;
import com.example.product.api.dto.SkuDTO;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/**
 * 订单域实现。
 *
 * <p>关键设计：
 * <ul>
 *     <li><b>价格只信服务端</b>：下单时通过 RPC 读取 SKU 售价与商品信息并写入 order_item 快照，
 *     客户端提交的价格一律忽略</li>
 *     <li><b>库存预占与补偿</b>：预占失败要显式释放本次已成功的部分，避免锁定库存泄漏。
 *     注意预占发生在 {@link #createOrder} 的本地事务<b>内部</b>（远程写操作会一直占着数据库连接），
 *     属于学习项目的取舍，生产更宜用本地预占表 + 消息驱动最终一致</li>
 *     <li><b>状态流转走条件更新</b>：所有状态变更都用「WHERE 原状态 = X」的 UPDATE + 影响行数判定，
 *     天然抵御并发重复处理（重复回调、取消与支付竞争）</li>
 *     <li><b>取消类副作用后置到事务提交之后</b>：只有「释放库存」通过
 *     {@code TransactionSynchronization#afterCommit} 挪到提交后，因为它是不可回滚的远程副作用</li>
 * </ul>
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class OrderServiceImpl implements OrderService {

    private static final int DEFAULT_PAGE_SIZE = 10;

    private static final int MAX_PAGE_SIZE = 50;

    private final OrderMapper orderMapper;

    private final OrderItemMapper orderItemMapper;

    private final PaymentOrderMapper paymentOrderMapper;

    private final ProductRpcClient productRpcClient;

    private final OrderLookup orderLookup;

    private final OrderProperties properties;

    /**
     * 创建订单。
     *
     * <p><b>已知取舍：没有幂等键</b>。客户端超时重试会真的产生两张订单并预占两次库存；
     * 生产环境应当增加 {@code Idempotency-Key}（或客户端自带的订单号）+ 唯一索引，
     * 重试时直接返回首次创建的结果。
     */
    @Override
    @Transactional(rollbackFor = Exception.class)
    public OrderVO createOrder(String uid, CreateOrderRequest request) {
        String currentUid = requireUid(uid);
        List<OrderItemRequest> itemRequests = normalizeItems(request);

        // 1. 通过 RPC 读取商品 / SKU，做可售性校验并计算金额（价格一律以 product-service 为准）
        List<OrderItem> items = new ArrayList<>(itemRequests.size());
        long totalAmount = 0L;
        // 归一化后的再次去重：normalizeItems 只能按请求原文比较，
        // 而 “12” 与 “S12abc” 可能指向同一个 SKU（EntityLookup 也接受内部数字 ID），
        // 所以拿到 RPC 返回的业务编号后再去一次重
        Set<String> resolvedSkuIds = new HashSet<>(itemRequests.size());
        for (OrderItemRequest itemRequest : itemRequests) {
            String skuId = itemRequest.skuId();
            SkuDTO sku = productRpcClient.getSku(skuId);
            if (sku == null) {
                throw new BizException(ErrorCode.SKU_NOT_FOUND, "SKU不存在: " + skuId);
            }
            if (!resolvedSkuIds.add(sku.getSkuId())) {
                throw new BizException(ErrorCode.INVALID_PARAM, "同一SKU不能重复提交: " + sku.getSkuId());
            }
            if (!SkuStatus.ENABLED.is(sku.getStatus())) {
                throw new BizException(ErrorCode.SKU_STATUS_CONFLICT, "SKU当前不可售: " + skuId);
            }
            if (sku.getPrice() == null) {
                throw new BizException(ErrorCode.SKU_STATUS_CONFLICT, "SKU未设置价格: " + skuId);
            }
            ProductDTO product = productRpcClient.getProduct(sku.getProductId());
            if (product == null) {
                throw new BizException(ErrorCode.PRODUCT_NOT_FOUND, "商品不存在: " + sku.getProductId());
            }
            if (!ProductStatus.ON_SHELF.is(product.getStatus())) {
                throw new BizException(ErrorCode.PRODUCT_STATUS_CONFLICT, "商品未上架: " + product.getName());
            }

            long itemTotal = multiply(sku.getPrice(), itemRequest.quantity());
            totalAmount = add(totalAmount, itemTotal);

            OrderItem item = new OrderItem();
            item.setProductId(product.getProductId());
            // 存 product-service 返回的业务编号，而不是请求里的原文：
            // 请求里可能传的是内部数字 ID，原样入库会让同一 SKU 在库里出现两种写法
            item.setSkuId(sku.getSkuId());
            item.setProductName(product.getName());
            item.setSkuName(sku.getName());
            item.setUnitPrice(sku.getPrice());
            item.setQuantity(itemRequest.quantity());
            item.setTotalAmount(itemTotal);
            items.add(item);
        }

        // 2. 落库：先写订单拿到内部主键，再批量写明细
        Order order = new Order();
        order.setOrderId(IdGenerator.nextOrderId());
        order.setUid(currentUid);
        order.setStatus(OrderStatus.WAIT_PAY.getCode());
        order.setTotalAmount(totalAmount);
        order.setExpireAt(LocalDateTime.now().plusMinutes(properties.getExpireMinutes()));
        orderMapper.insert(order);
        items.forEach(item -> item.setOrderId(order.getId()));
        orderItemMapper.batchInsert(items);

        // 3. 预占库存（失败需补偿已成功的预占）
        reserveStock(items);

        log.info("创建订单成功: orderId={}, uid={}, skuCount={}, totalAmount={}, expireAt={}",
                order.getOrderId(), currentUid, items.size(), totalAmount, order.getExpireAt());
        return OrderVO.of(orderMapper.selectById(order.getId()), items);
    }

    @Override
    @Transactional(readOnly = true)
    public OrderVO getOrder(String uid, String orderId) {
        Order order = orderLookup.requireMine(orderId, requireUid(uid));
        return OrderVO.of(order, orderItemMapper.selectByOrderId(order.getId()));
    }

    @Override
    @Transactional(readOnly = true)
    public PageVO<OrderVO> listMyOrders(String uid, Integer status, int page, int size) {
        String currentUid = requireUid(uid);
        if (status != null) {
            // 提前校验，非法状态直接返回 40001，而不是查出空列表
            OrderStatus.of(status);
        }
        int currentPage = Math.max(page, 1);
        int pageSize = size <= 0 ? DEFAULT_PAGE_SIZE : Math.min(size, MAX_PAGE_SIZE);

        long total = orderMapper.countByUid(currentUid, status);
        if (total == 0) {
            return PageVO.of(currentPage, pageSize, 0L, List.of());
        }
        List<Order> orders = orderMapper.selectByUid(currentUid, status, (currentPage - 1) * pageSize, pageSize);
        log.debug("查询我的订单: uid={}, status={}, page={}, size={}, total={}", currentUid, status, currentPage,
                pageSize, total);
        return PageVO.of(currentPage, pageSize, total, assemble(orders));
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public OrderVO cancelOrder(String uid, String orderId) {
        Order order = orderLookup.requireMine(orderId, requireUid(uid));
        if (!OrderStatus.WAIT_PAY.is(order.getStatus())) {
            throw new BizException(ErrorCode.ORDER_STATUS_CONFLICT, "订单当前状态不允许取消: "
                    + OrderStatus.of(order.getStatus()).getDescription());
        }
        if (orderMapper.markCancelled(order.getId(), LocalDateTime.now()) == 0) {
            // 并发下已被超时任务取消 / 用户重复点击
            throw new BizException(ErrorCode.ORDER_STATUS_CONFLICT, "订单状态已变更，请刷新后重试");
        }
        List<OrderItem> items = orderItemMapper.selectByOrderId(order.getId());
        paymentOrderMapper.closeWaitPayByOrderId(order.getOrderId(), PaymentStatus.CLOSED.getCode());
        releaseAfterCommit(items, "用户取消订单");

        log.info("取消订单成功: orderId={}, uid={}, itemCount={}", order.getOrderId(), order.getUid(), items.size());
        return OrderVO.of(orderMapper.selectById(order.getId()), items);
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public OrderVO markPaid(String orderId, LocalDateTime paidAt) {
        Order order = orderLookup.require(orderId);
        if (orderMapper.markPaid(order.getId(), paidAt) == 0) {
            // 走到这里说明支付成功，但订单已被取消：属于资金与订单不一致，必须人工核对
            log.error("支付成功但订单状态不允许流转，需人工核对退款: orderId={}, uid={}, status={}",
                    orderId, order.getUid(), order.getStatus());
            return OrderVO.of(orderMapper.selectById(order.getId()), orderItemMapper.selectByOrderId(order.getId()));
        }
        log.info("订单支付成功: orderId={}, uid={}, totalAmount={}, paidAt={}",
                orderId, order.getUid(), order.getTotalAmount(), paidAt);

        // 权益发放：失败时订单停留在 PAID，由补偿任务重试（故此处不抛异常、不影响支付结果）
        completePaidOrder(orderId);
        return OrderVO.of(orderMapper.selectById(order.getId()), orderItemMapper.selectByOrderId(order.getId()));
    }

    /**
     * {@inheritDoc}
     *
     * <p><b>关于事务</b>：本方法自身没有 {@code @Transactional}，但它的两个调用方都处在事务中
     * （{@link #markPaid} 的同类自调用不会走上代理，{@code PaymentServiceImpl.handleCallback} 上有显式事务注解），
     * 所以下面的 {@code confirmStock} 实际是<b>在数据库事务内</b>发出的远程调用——早期注释写反了，请以此为准。
     *
     * <p>幂等由 {@code markCompleted} 的条件更新保证，重复执行只会成功一次；
     * 副作用顺序是「先确认库存、再改订单状态」，因此中途失败会停在 PAID，由补偿任务重试。
     *
     * <p>改进方向：把远程调用用 {@code TransactionSynchronization#afterCommit} 或本地消息表移出事务。
     */
    @Override
    public boolean completePaidOrder(String orderId) {
        Order order = orderMapper.selectByOrderId(orderId);
        if (order == null || !OrderStatus.PAID.is(order.getStatus())) {
            return false;
        }
        List<OrderItem> items = orderItemMapper.selectByOrderId(order.getId());
        for (OrderItem item : items) {
            if (!productRpcClient.confirmStock(item.getSkuId(), item.getQuantity())) {
                log.warn("权益发放未完成，订单停留在已支付等待补偿: orderId={}, skuId={}, quantity={}",
                        orderId, item.getSkuId(), item.getQuantity());
                return false;
            }
        }
        boolean completed = orderMapper.markCompleted(order.getId(), LocalDateTime.now()) > 0;
        if (completed) {
            log.info("订单权益发放完成: orderId={}, uid={}, itemCount={}", orderId, order.getUid(), items.size());
        }
        return completed;
    }

    @Override
    @Transactional(rollbackFor = Exception.class)
    public boolean cancelExpiredOrder(String orderId) {
        Order order = orderMapper.selectByOrderId(orderId);
        if (order == null || !OrderStatus.WAIT_PAY.is(order.getStatus())) {
            return false;
        }
        LocalDateTime now = LocalDateTime.now();
        if (order.getExpireAt() == null || order.getExpireAt().isAfter(now)) {
            return false;
        }
        if (orderMapper.markCancelled(order.getId(), now) == 0) {
            // 与用户主动取消 / 支付回调并发，本次无需处理
            return false;
        }
        List<OrderItem> items = orderItemMapper.selectByOrderId(order.getId());
        paymentOrderMapper.closeWaitPayByOrderId(order.getOrderId(), PaymentStatus.CLOSED.getCode());
        releaseAfterCommit(items, "订单超时取消");

        log.info("订单超时自动取消: orderId={}, uid={}, expireAt={}, itemCount={}",
                order.getOrderId(), order.getUid(), order.getExpireAt(), items.size());
        return true;
    }

    @Override
    public List<String> findExpiredOrderIds(LocalDateTime now, int limit) {
        return orderMapper.selectExpired(now, limit).stream().map(Order::getOrderId).toList();
    }

    @Override
    public List<String> findCompletableOrderIds(LocalDateTime paidBefore, int limit) {
        return orderMapper.selectPaidBefore(paidBefore, limit).stream().map(Order::getOrderId).toList();
    }

    /** 校验并归一化请求明细：去空格、拦截重复 SKU */
    private List<OrderItemRequest> normalizeItems(CreateOrderRequest request) {
        if (request == null || request.items() == null || request.items().isEmpty()) {
            throw new BizException(ErrorCode.INVALID_PARAM, "订单商品不能为空");
        }
        Set<String> skuIds = new HashSet<>(request.items().size());
        List<OrderItemRequest> normalized = new ArrayList<>(request.items().size());
        for (OrderItemRequest item : request.items()) {
            String skuId = item.skuId() == null ? "" : item.skuId().trim();
            if (skuId.isEmpty()) {
                throw new BizException(ErrorCode.INVALID_PARAM, "SKU编号不能为空");
            }
            if (!skuIds.add(skuId)) {
                throw new BizException(ErrorCode.INVALID_PARAM, "同一SKU不能重复提交: " + skuId);
            }
            normalized.add(new OrderItemRequest(skuId, item.quantity()));
        }
        return normalized;
    }

    /** 预占库存：任一 SKU 失败则补偿释放本次已成功的预占，再抛出业务异常 */
    private void reserveStock(List<OrderItem> items) {
        List<OrderItem> reserved = new ArrayList<>(items.size());
        try {
            for (OrderItem item : items) {
                if (!productRpcClient.reserveStock(item.getSkuId(), item.getQuantity())) {
                    throw new BizException(ErrorCode.STOCK_NOT_ENOUGH,
                            "库存不足: " + item.getSkuName() + " x" + item.getQuantity());
                }
                reserved.add(item);
            }
        } catch (RuntimeException ex) {
            releaseQuietly(reserved, "创建订单失败补偿");
            throw ex;
        }
    }

    /**
     * 事务提交后再释放库存。
     *
     * <p>释放库存是远程副作用且不可回滚：若在事务内执行而事务最终回滚，订单仍是待支付状态，
     * 下一次取消会再次释放，导致可用库存被超发。因此统一挪到 commit 之后执行。
     */
    private void releaseAfterCommit(List<OrderItem> items, String scene) {
        if (!TransactionSynchronizationManager.isSynchronizationActive()) {
            releaseQuietly(items, scene);
            return;
        }
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                releaseQuietly(items, scene);
            }
        });
    }

    /**
     * 尽力而为地释放库存：失败只告警，不能让「取消订单」这个主流程失败。
     *
     * <p>日志里的 {@code orderDbId} 是 {@code order}.id（自增主键）；
     * 早期把它写成 {@code orderId=}，会让人以为是对外的业务编号 {@code order.order_id}，
     * 排查时按错字段去查订单。
     */
    private void releaseQuietly(List<OrderItem> items, String scene) {
        for (OrderItem item : items) {
            try {
                if (!productRpcClient.releaseStock(item.getSkuId(), item.getQuantity())) {
                    log.error("{}: 释放库存未生效，需对账补偿: orderDbId={}, skuId={}, quantity={}",
                            scene, item.getOrderId(), item.getSkuId(), item.getQuantity());
                }
            } catch (RuntimeException ex) {
                log.error("{}: 释放库存调用异常，需对账补偿: orderDbId={}, skuId={}, quantity={}",
                        scene, item.getOrderId(), item.getSkuId(), item.getQuantity(), ex);
            }
        }
    }

    /** 批量装配订单明细，避免列表接口 N+1 查询 */
    private List<OrderVO> assemble(List<Order> orders) {
        if (orders.isEmpty()) {
            return List.of();
        }
        List<Long> orderIds = orders.stream().map(Order::getId).toList();
        Map<Long, List<OrderItem>> itemsByOrderId = orderItemMapper.selectByOrderIds(orderIds).stream()
                .collect(Collectors.groupingBy(OrderItem::getOrderId));
        return orders.stream()
                .map(order -> OrderVO.of(order, itemsByOrderId.getOrDefault(order.getId(), List.of())))
                .toList();
    }

    private String requireUid(String uid) {
        if (uid == null || uid.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "缺少用户身份信息，请通过网关登录后访问");
        }
        return uid.trim();
    }

    private long multiply(long price, int quantity) {
        try {
            return Math.multiplyExact(price, (long) quantity);
        } catch (ArithmeticException ex) {
            throw new BizException(ErrorCode.INVALID_PARAM, "订单金额超出可计算范围");
        }
    }

    private long add(long left, long right) {
        try {
            return Math.addExact(left, right);
        } catch (ArithmeticException ex) {
            throw new BizException(ErrorCode.INVALID_PARAM, "订单金额超出可计算范围");
        }
    }
}
