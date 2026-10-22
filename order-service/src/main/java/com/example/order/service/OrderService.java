package com.example.order.service;

import com.example.order.common.PageVO;
import com.example.order.dto.request.CreateOrderRequest;
import com.example.order.dto.response.OrderVO;
import java.time.LocalDateTime;
import java.util.List;

/**
 * 订单域服务：承载订单状态机的全部流转。
 */
public interface OrderService {

    /** 创建订单：RPC 校验商品/SKU 并快照价格，落库后预占库存 */
    OrderVO createOrder(String uid, CreateOrderRequest request);

    /** 查询订单详情，仅限本人订单 */
    OrderVO getOrder(String uid, String orderId);

    /** 分页查询我的订单，status 为空表示全部状态 */
    PageVO<OrderVO> listMyOrders(String uid, Integer status, int page, int size);

    /** 取消订单：WAIT_PAY → CANCELLED，并释放锁定库存、关闭未支付的支付单 */
    OrderVO cancelOrder(String uid, String orderId);

    /**
     * 支付成功回调：WAIT_PAY → PAID，随后尝试权益发放（PAID → COMPLETED）。
     *
     * <p>调用方为支付服务，无用户身份；幂等：订单已不是待支付状态时不做任何变更。
     */
    OrderVO markPaid(String orderId, LocalDateTime paidAt);

    /**
     * 权益发放：确认库存消耗（锁定库存 → 已扣减），全部成功才把订单置为 COMPLETED。
     *
     * @return 是否发放完成；false 表示订单停在 PAID，等待补偿任务重试
     */
    boolean completePaidOrder(String orderId);

    /**
     * 超时取消单个订单：WAIT_PAY 且已过 {@code expire_at} → CANCELLED。
     *
     * @return 本次是否真的取消成功（已取消 / 已支付 / 未到期都返回 false）
     */
    boolean cancelExpiredOrder(String orderId);

    /** 扫描已超时未支付的订单业务编号（供定时任务使用） */
    List<String> findExpiredOrderIds(LocalDateTime now, int limit);

    /** 扫描已支付但权益未发放完成的订单业务编号（供补偿任务使用） */
    List<String> findCompletableOrderIds(LocalDateTime paidBefore, int limit);
}
