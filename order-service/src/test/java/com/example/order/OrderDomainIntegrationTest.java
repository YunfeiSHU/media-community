package com.example.order;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.BDDMockito.given;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;
import com.example.order.domain.entity.Order;
import com.example.order.domain.entity.OrderItem;
import com.example.order.domain.entity.PaymentOrder;
import com.example.order.domain.enums.CallbackStatus;
import com.example.order.domain.enums.OrderStatus;
import com.example.order.domain.enums.PaymentStatus;
import com.example.order.dto.request.CreateOrderRequest;
import com.example.order.dto.request.CreateOrderRequest.OrderItemRequest;
import com.example.order.dto.request.CreatePaymentRequest;
import com.example.order.dto.request.PaymentCallbackRequest;
import com.example.order.dto.response.OrderVO;
import com.example.order.dto.response.PaymentVO;
import com.example.order.mapper.OrderItemMapper;
import com.example.order.mapper.OrderMapper;
import com.example.order.mapper.PaymentOrderMapper;
import com.example.order.mapper.PaymentRecordMapper;
import com.example.order.service.OrderService;
import com.example.order.service.PaymentService;
import com.example.order.support.PaymentSigner;
import com.example.order.support.ProductRpcClient;
import com.example.product.api.dto.ProductDTO;
import com.example.product.api.dto.SkuDTO;
import java.time.LocalDateTime;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.test.context.ActiveProfiles;

/**
 * 领域集成测试：直接调用 Service 覆盖 HTTP 层难以构造的场景。
 *
 * <p><b>刻意不加 {@code @Transactional}</b>：取消类操作的库存释放被安排在「事务提交之后」执行，
 * 若测试用事务包裹，释放动作永远不会触发，也就测不到真实行为。因此各用例使用彼此独立的
 * UID / 订单编号，避免数据互相干扰。
 */
@SpringBootTest
@ActiveProfiles("test")
class OrderDomainIntegrationTest {

    private static final String PRODUCT_ID = "PROD-DOMAIN-1";

    private static final String SKU_ID = "SKU-DOMAIN-1";

    private static final String SKU_ID_2 = "SKU-DOMAIN-2";

    private static final long SKU_PRICE = 9900L;

    @Autowired
    private OrderService orderService;

    @Autowired
    private PaymentService paymentService;

    @Autowired
    private PaymentSigner paymentSigner;

    @Autowired
    private OrderMapper orderMapper;

    @Autowired
    private OrderItemMapper orderItemMapper;

    @Autowired
    private PaymentOrderMapper paymentOrderMapper;

    @Autowired
    private PaymentRecordMapper paymentRecordMapper;

    @MockBean
    private ProductRpcClient productRpcClient;

    @BeforeEach
    void setUp() {
        given(productRpcClient.getSku(SKU_ID)).willReturn(sku(SKU_ID, SKU_PRICE));
        given(productRpcClient.getSku(SKU_ID_2)).willReturn(sku(SKU_ID_2, 100L));
        given(productRpcClient.getProduct(PRODUCT_ID)).willReturn(product());
        given(productRpcClient.reserveStock(anyString(), anyInt())).willReturn(true);
        given(productRpcClient.confirmStock(anyString(), anyInt())).willReturn(true);
        given(productRpcClient.releaseStock(anyString(), anyInt())).willReturn(true);
    }

    @Test
    @DisplayName("领域：创建订单会写入价格快照并预占库存")
    void createOrderSnapshotsPriceAndReservesStock() {
        OrderVO order = orderService.createOrder("u-domain-create", request(SKU_ID, 2));

        assertThat(order.status()).isEqualTo(OrderStatus.WAIT_PAY.getCode());
        // 金额只由服务端计算：9900 * 2
        assertThat(order.totalAmount()).isEqualTo(SKU_PRICE * 2);
        assertThat(order.expireAt()).isAfter(LocalDateTime.now());
        assertThat(order.items()).hasSize(1);
        assertThat(order.items().get(0).unitPrice()).isEqualTo(SKU_PRICE);
        assertThat(order.items().get(0).totalAmount()).isEqualTo(SKU_PRICE * 2);
        assertThat(order.items().get(0).skuName()).isEqualTo("规格-" + SKU_ID);
        assertThat(order.items().get(0).productName()).isEqualTo("领域商品");

        verify(productRpcClient).reserveStock(SKU_ID, 2);
    }

    @Test
    @DisplayName("领域：库存不足时订单不落库，并补偿释放已成功的预占")
    void createOrderReleasesReservedStockWhenStockNotEnough() {
        given(productRpcClient.reserveStock(SKU_ID, 1)).willReturn(true);
        given(productRpcClient.reserveStock(SKU_ID_2, 1)).willReturn(false);

        String uid = "u-domain-stock";
        assertThatThrownBy(() -> orderService.createOrder(uid, new CreateOrderRequest(List.of(
                new OrderItemRequest(SKU_ID, 1),
                new OrderItemRequest(SKU_ID_2, 1)))))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.STOCK_NOT_ENOUGH);

        // 订单与明细都被回滚
        assertThat(orderMapper.countByUid(uid, null)).isZero();
        // 第一个 SKU 的预占被补偿释放，失败的 SKU 不需要释放
        verify(productRpcClient).releaseStock(SKU_ID, 1);
        verify(productRpcClient, never()).releaseStock(SKU_ID_2, 1);
    }

    @Test
    @DisplayName("领域：订单超时取消会关闭支付单并释放锁定库存")
    void cancelExpiredOrder() {
        String uid = "u-domain-timeout";
        Order order = insertWaitPayOrder(uid, LocalDateTime.now().minusMinutes(1));
        orderItemMapper.batchInsert(List.of(orderItem(order.getId(), SKU_ID, 2)));
        PaymentOrder payment = insertWaitPayPayment(order);

        // 已到期 → 可被任务扫描到
        assertThat(orderService.findExpiredOrderIds(LocalDateTime.now(), 100)).contains(order.getOrderId());

        assertThat(orderService.cancelExpiredOrder(order.getOrderId())).isTrue();
        Order cancelled = orderMapper.selectByOrderId(order.getOrderId());
        assertThat(cancelled.getStatus()).isEqualTo(OrderStatus.CANCELLED.getCode());
        assertThat(cancelled.getCancelledAt()).isNotNull();
        assertThat(paymentOrderMapper.selectByPaymentId(payment.getPaymentId()).getStatus())
                .isEqualTo(PaymentStatus.CLOSED.getCode());
        // 库存释放发生在事务提交之后
        verify(productRpcClient).releaseStock(SKU_ID, 2);

        // 重复处理是幂等的：已取消的订单不再释放库存
        assertThat(orderService.cancelExpiredOrder(order.getOrderId())).isFalse();
        verify(productRpcClient, times(1)).releaseStock(SKU_ID, 2);

        // 未到期的订单不会被处理
        Order notExpired = insertWaitPayOrder(uid, LocalDateTime.now().plusMinutes(30));
        assertThat(orderService.cancelExpiredOrder(notExpired.getOrderId())).isFalse();
    }

    @Test
    @DisplayName("领域：支付回调幂等，重复回调不会重复发放权益")
    void paymentCallbackIsIdempotent() {
        String uid = "u-domain-idempotent";
        OrderVO order = orderService.createOrder(uid, request(SKU_ID, 2));
        PaymentVO payment = paymentService.createPayment(uid, order.orderId(),
                new CreatePaymentRequest(1));

        paymentService.handleCallback(callback(payment, "TX-IDEMPOTENT-1", CallbackStatus.SUCCESS));

        assertThat(paymentOrderMapper.selectByPaymentId(payment.paymentId()).getStatus())
                .isEqualTo(PaymentStatus.PAID.getCode());
        Order paid = orderMapper.selectByOrderId(order.orderId());
        assertThat(paid.getStatus()).isEqualTo(OrderStatus.COMPLETED.getCode());
        assertThat(paid.getPaidAt()).isNotNull();
        assertThat(paid.getCompletedAt()).isNotNull();

        // 渠道未收到应答会重试：第二次回调不改变任何状态，也不重复确认库存
        paymentService.handleCallback(callback(payment, "TX-IDEMPOTENT-1", CallbackStatus.SUCCESS));
        assertThat(orderMapper.selectByOrderId(order.orderId()).getStatus())
                .isEqualTo(OrderStatus.COMPLETED.getCode());
        verify(productRpcClient, times(1)).confirmStock(SKU_ID, 2);
        // 两次回调都留了痕，便于对账
        assertThat(paymentRecordMapper.selectByPaymentId(payment.paymentId())).hasSize(2);
    }

    @Test
    @DisplayName("领域：权益发放失败时订单停在已支付，补偿后可完成")
    void orderStaysPaidWhenBenefitGrantFails() {
        given(productRpcClient.confirmStock(anyString(), anyInt())).willReturn(false);

        String uid = "u-domain-compensate";
        OrderVO order = orderService.createOrder(uid, request(SKU_ID, 2));
        PaymentVO payment = paymentService.createPayment(uid, order.orderId(),
                new CreatePaymentRequest(1));

        paymentService.handleCallback(callback(payment, "TX-COMPENSATE-1", CallbackStatus.SUCCESS));

        // 钱已经收到，订单先停在已支付，等待补偿
        assertThat(paymentOrderMapper.selectByPaymentId(payment.paymentId()).getStatus())
                .isEqualTo(PaymentStatus.PAID.getCode());
        assertThat(orderMapper.selectByOrderId(order.orderId()).getStatus())
                .isEqualTo(OrderStatus.PAID.getCode());
        assertThat(orderService.findCompletableOrderIds(LocalDateTime.now(), 100))
                .contains(order.orderId());

        // 库存服务恢复后，补偿任务把订单推进到已完成
        given(productRpcClient.confirmStock(anyString(), anyInt())).willReturn(true);
        assertThat(orderService.completePaidOrder(order.orderId())).isTrue();
        assertThat(orderMapper.selectByOrderId(order.orderId()).getStatus())
                .isEqualTo(OrderStatus.COMPLETED.getCode());
    }

    @Test
    @DisplayName("领域：支付失败回调把支付单置为失败，用户可重新发起支付")
    void paymentFailedCallbackAllowsRetry() {
        String uid = "u-domain-failed";
        OrderVO order = orderService.createOrder(uid, request(SKU_ID, 1));
        PaymentVO first = paymentService.createPayment(uid, order.orderId(), new CreatePaymentRequest(1));

        paymentService.handleCallback(callback(first, "TX-FAILED-1", CallbackStatus.FAILED));

        assertThat(paymentOrderMapper.selectByPaymentId(first.paymentId()).getStatus())
                .isEqualTo(PaymentStatus.FAILED.getCode());
        // 订单仍是待支付，用户可以换渠道重试
        assertThat(orderMapper.selectByOrderId(order.orderId()).getStatus())
                .isEqualTo(OrderStatus.WAIT_PAY.getCode());

        PaymentVO second = paymentService.createPayment(uid, order.orderId(), new CreatePaymentRequest(2));
        assertThat(second.paymentId()).isNotEqualTo(first.paymentId());
        assertThat(second.paymentChannel()).isEqualTo(2);
    }

    @Test
    @DisplayName("领域：回调金额与支付单不一致时拒绝处理")
    void callbackWithMismatchedAmountIsRejected() {
        String uid = "u-domain-amount";
        OrderVO order = orderService.createOrder(uid, request(SKU_ID, 1));
        PaymentVO payment = paymentService.createPayment(uid, order.orderId(), new CreatePaymentRequest(1));

        PaymentCallbackRequest callback = callback(payment.paymentId(), "TX-AMOUNT-1", 1L,
                CallbackStatus.SUCCESS, 1);
        assertThatThrownBy(() -> paymentService.handleCallback(callback))
                .isInstanceOf(BizException.class)
                .extracting(ex -> ((BizException) ex).getErrorCode())
                .isEqualTo(ErrorCode.PAYMENT_AMOUNT_MISMATCH);

        assertThat(paymentOrderMapper.selectByPaymentId(payment.paymentId()).getStatus())
                .isEqualTo(PaymentStatus.WAIT_PAY.getCode());
        assertThat(orderMapper.selectByOrderId(order.orderId()).getStatus())
                .isEqualTo(OrderStatus.WAIT_PAY.getCode());
    }

    private CreateOrderRequest request(String skuId, int quantity) {
        return new CreateOrderRequest(List.of(new OrderItemRequest(skuId, quantity)));
    }

    /** 构造合法签名的回调请求 */
    private PaymentCallbackRequest callback(PaymentVO payment, String transactionId, CallbackStatus tradeStatus) {
        return callback(payment.paymentId(), transactionId, payment.amount(), tradeStatus,
                payment.paymentChannel());
    }

    private PaymentCallbackRequest callback(String paymentId, String transactionId, long amount,
                                            CallbackStatus tradeStatus, int channel) {
        PaymentCallbackRequest unsigned = new PaymentCallbackRequest(paymentId, transactionId, channel, tradeStatus,
                amount, "");
        return new PaymentCallbackRequest(paymentId, transactionId, channel, tradeStatus, amount,
                paymentSigner.sign(unsigned));
    }

    private Order insertWaitPayOrder(String uid, LocalDateTime expireAt) {
        Order order = new Order();
        order.setOrderId("O-DOMAIN-" + System.nanoTime());
        order.setUid(uid);
        order.setStatus(OrderStatus.WAIT_PAY.getCode());
        order.setTotalAmount(SKU_PRICE * 2);
        order.setExpireAt(expireAt);
        orderMapper.insert(order);
        return order;
    }

    private OrderItem orderItem(Long orderId, String skuId, int quantity) {
        OrderItem item = new OrderItem();
        item.setOrderId(orderId);
        item.setProductId(PRODUCT_ID);
        item.setSkuId(skuId);
        item.setProductName("领域商品");
        item.setSkuName("规格-" + skuId);
        item.setUnitPrice(SKU_PRICE);
        item.setQuantity(quantity);
        item.setTotalAmount(SKU_PRICE * quantity);
        return item;
    }

    private PaymentOrder insertWaitPayPayment(Order order) {
        PaymentOrder payment = new PaymentOrder();
        payment.setPaymentId("P-DOMAIN-" + System.nanoTime());
        payment.setOrderId(order.getOrderId());
        payment.setUid(order.getUid());
        payment.setAmount(order.getTotalAmount());
        payment.setStatus(PaymentStatus.WAIT_PAY.getCode());
        payment.setPaymentChannel(1);
        payment.setExpireAt(order.getExpireAt());
        paymentOrderMapper.insert(payment);
        return payment;
    }

    private SkuDTO sku(String skuId, long price) {
        SkuDTO dto = new SkuDTO();
        dto.setSkuId(skuId);
        dto.setProductId(PRODUCT_ID);
        dto.setName("规格-" + skuId);
        dto.setPrice(price);
        dto.setStatus(1);
        return dto;
    }

    private ProductDTO product() {
        ProductDTO dto = new ProductDTO();
        dto.setProductId(PRODUCT_ID);
        dto.setUid("u-domain-seller");
        dto.setName("领域商品");
        dto.setProductType(1);
        dto.setStatus(1);
        return dto;
    }
}
