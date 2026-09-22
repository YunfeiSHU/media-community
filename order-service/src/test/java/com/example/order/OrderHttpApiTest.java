package com.example.order;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.BDDMockito.given;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.example.order.domain.enums.CallbackStatus;
import com.example.order.domain.enums.OrderStatus;
import com.example.order.domain.enums.PaymentStatus;
import com.example.order.dto.request.PaymentCallbackRequest;
import com.example.order.support.PaymentSigner;
import com.example.order.support.ProductRpcClient;
import com.example.product.api.dto.ProductDTO;
import com.example.product.api.dto.SkuDTO;
import com.jayway.jsonpath.JsonPath;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.springframework.test.web.servlet.request.MockHttpServletRequestBuilder;
import org.springframework.transaction.annotation.Transactional;

/**
 * HTTP 接口契约测试：覆盖设计文档约定的全部路由，并校验统一响应结构与错误码映射。
 *
 * <p>RPC 依赖用 {@code @MockBean} 整体替换，测试不依赖 product-service 真实运行。
 */
@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
@Transactional
class OrderHttpApiTest {

    private static final String UID_HEADER = "uid";

    private static final String UID = "u-1001";

    private static final String OTHER_UID = "u-2002";

    private static final String PRODUCT_ID = "PROD-HTTP-1";

    private static final String SKU_ID = "SKU-HTTP-1";

    private static final String SKU_ID_2 = "SKU-HTTP-2";

    private static final long SKU_PRICE = 9900L;

    private static final long SKU_PRICE_2 = 100L;

    /** 回调报文中的支付渠道：与创建支付单时保持一致 */
    private static final int CHANNEL = 1;

    @Autowired
    private MockMvc mockMvc;

    @Autowired
    private PaymentSigner paymentSigner;

    @MockBean
    private ProductRpcClient productRpcClient;

    @BeforeEach
    void setUp() {
        given(productRpcClient.getSku(SKU_ID)).willReturn(sku(SKU_ID, SKU_PRICE));
        given(productRpcClient.getSku(SKU_ID_2)).willReturn(sku(SKU_ID_2, SKU_PRICE_2));
        given(productRpcClient.getProduct(PRODUCT_ID)).willReturn(product());
        given(productRpcClient.reserveStock(anyString(), anyInt())).willReturn(true);
        given(productRpcClient.confirmStock(anyString(), anyInt())).willReturn(true);
        given(productRpcClient.releaseStock(anyString(), anyInt())).willReturn(true);
    }

    @Test
    @DisplayName("HTTP：创建订单 / 查询详情 / 我的订单 / 创建支付单 / 支付回调 全链路")
    void fullHttpFlow() throws Exception {
        // 1. 创建订单 POST /api/v1/orders
        String created = ok(post("/api/v1/orders").content("""
                {"items": [
                    {"skuId": "SKU-HTTP-1", "quantity": 2},
                    {"skuId": "SKU-HTTP-2", "quantity": 3}
                ]}
                """), UID);
        String orderId = read(created, "$.data.orderId");
        assertThat(orderId).startsWith("O");
        assertThat(read(created, "$.data.uid")).isEqualTo(UID);
        assertThat(readInt(created, "$.data.status")).isEqualTo(OrderStatus.WAIT_PAY.getCode());
        // 金额完全由服务端按快照算出：9900 * 2 + 100 * 3
        assertThat(readLong(created, "$.data.totalAmount")).isEqualTo(SKU_PRICE * 2 + SKU_PRICE_2 * 3);
        assertThat(readList(created, "$.data.items")).hasSize(2);
        assertThat(readLong(created, "$.data.items[0].unitPrice")).isEqualTo(SKU_PRICE);

        // 2. 查询订单详情 GET /api/v1/orders/{id}
        String detail = ok(get("/api/v1/orders/" + orderId), UID);
        assertThat(read(detail, "$.data.orderId")).isEqualTo(orderId);
        assertThat(read(detail, "$.data.statusName")).isEqualTo("待支付");

        // 3. 查询我的订单 GET /api/v1/orders
        String page = ok(get("/api/v1/orders"), UID);
        assertThat(readLong(page, "$.data.total")).isEqualTo(1L);
        assertThat(read(page, "$.data.list[0].orderId")).isEqualTo(orderId);
        assertThat(read(page, "$.data.list[0].items[1].skuId")).isEqualTo(SKU_ID_2);
        // 按状态过滤：待支付命中
        assertThat(readLong(ok(get("/api/v1/orders?status=0"), UID), "$.data.total")).isEqualTo(1L);
        // 按状态过滤：已完成不命中
        assertThat(readLong(ok(get("/api/v1/orders?status=2"), UID), "$.data.total")).isZero();

        // 4. 创建支付单 POST /api/v1/orders/{id}/pay
        String payment = ok(post("/api/v1/orders/" + orderId + "/pay").content("""
                {"paymentChannel": 2}
                """), UID);
        String paymentId = read(payment, "$.data.paymentId");
        assertThat(paymentId).startsWith("P");
        assertThat(readInt(payment, "$.data.status")).isEqualTo(PaymentStatus.WAIT_PAY.getCode());
        assertThat(read(payment, "$.data.paymentChannelName")).isEqualTo("支付宝");
        assertThat(readLong(payment, "$.data.amount")).isEqualTo(SKU_PRICE * 2 + SKU_PRICE_2 * 3);

        // 5. 支付回调 POST /api/v1/payments/callback（免登录）
        mockMvc.perform(post("/api/v1/payments/callback")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(callbackBody(paymentId, "WX-TX-0001", 2, SKU_PRICE * 2 + SKU_PRICE_2 * 3,
                                CallbackStatus.SUCCESS)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));

        // 6. 订单已支付且权益发放完成 → COMPLETED
        String afterPay = ok(get("/api/v1/orders/" + orderId), UID);
        assertThat(readInt(afterPay, "$.data.status")).isEqualTo(OrderStatus.COMPLETED.getCode());
        assertThat(read(afterPay, "$.data.paidAt")).isNotNull();
        assertThat(read(afterPay, "$.data.completedAt")).isNotNull();
    }

    @Test
    @DisplayName("HTTP：取消订单 WAIT_PAY → CANCELLED")
    void cancelOrder() throws Exception {
        String created = ok(post("/api/v1/orders").content(orderBody(SKU_ID, 1)), UID);
        String orderId = read(created, "$.data.orderId");
        // 顺带创建一张支付单，取消后应被关闭
        String paymentId = read(ok(post("/api/v1/orders/" + orderId + "/pay").content("""
                {"paymentChannel": 1}
                """), UID), "$.data.paymentId");

        String cancelled = ok(post("/api/v1/orders/" + orderId + "/cancel"), UID);
        assertThat(readInt(cancelled, "$.data.status")).isEqualTo(OrderStatus.CANCELLED.getCode());
        assertThat(read(cancelled, "$.data.cancelledAt")).isNotNull();

        // 取消后不可再次取消
        expect(post("/api/v1/orders/" + orderId + "/cancel"), UID, 409, 40901);
        // 取消后不可支付
        expect(post("/api/v1/orders/" + orderId + "/pay").content("""
                {"paymentChannel": 1}
                """), UID, 409, 40901);
        assertThat(paymentId).startsWith("P");
    }

    @Test
    @DisplayName("HTTP：创建支付单幂等（复用未支付的支付单）")
    void createPaymentIsIdempotent() throws Exception {
        String orderId = read(ok(post("/api/v1/orders").content(orderBody(SKU_ID, 1)), UID), "$.data.orderId");
        String first = ok(post("/api/v1/orders/" + orderId + "/pay").content("""
                {"paymentChannel": 1}
                """), UID);
        String second = ok(post("/api/v1/orders/" + orderId + "/pay").content("""
                {"paymentChannel": 2}
                """), UID);
        assertThat(read(second, "$.data.paymentId")).isEqualTo(read(first, "$.data.paymentId"));
    }

    @Test
    @DisplayName("HTTP：错误码与状态码映射")
    void errorResponses() throws Exception {
        // 缺少身份头（未经网关）→ 401 / 40101
        expect(post("/api/v1/orders").content(orderBody(SKU_ID, 1)), null, 401, 40101);

        // 订单不存在 → 404 / 40401
        expect(get("/api/v1/orders/NOT_EXIST_ORDER"), UID, 404, 40401);

        // 参数校验失败 → 400 / 40001
        expect(post("/api/v1/orders").content("""
                {"items": []}
                """), UID, 400, 40001);

        // 同一 SKU 重复提交 → 400 / 40001
        expect(post("/api/v1/orders").content("""
                {"items": [
                    {"skuId": "SKU-HTTP-1", "quantity": 1},
                    {"skuId": "SKU-HTTP-1", "quantity": 2}
                ]}
                """), UID, 400, 40001);

        // SKU 不存在 → 404 / 40404
        expect(post("/api/v1/orders").content(orderBody("SKU-UNKNOWN", 1)), UID, 404, 40404);

        // 库存不足 → 409 / 40904
        given(productRpcClient.reserveStock(SKU_ID, 5)).willReturn(false);
        expect(post("/api/v1/orders").content(orderBody(SKU_ID, 5)), UID, 409, 40904);

        // 查询他人订单 → 404 / 40401（不泄露订单是否存在）
        String orderId = read(ok(post("/api/v1/orders").content(orderBody(SKU_ID, 1)), UID), "$.data.orderId");
        expect(get("/api/v1/orders/" + orderId), OTHER_UID, 404, 40401);
        expect(post("/api/v1/orders/" + orderId + "/cancel"), OTHER_UID, 404, 40401);

        // 支付渠道非法 → 400 / 40001
        expect(post("/api/v1/orders/" + orderId + "/pay").content("""
                {"paymentChannel": 99}
                """), UID, 400, 40001);

        // 支付回调验签失败 → 400 / 40002
        String paymentId = read(ok(post("/api/v1/orders/" + orderId + "/pay").content("""
                {"paymentChannel": 1}
                """), UID), "$.data.paymentId");
        mockMvc.perform(post("/api/v1/payments/callback")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(callbackBody(paymentId, "WX-TX-BAD", CHANNEL, 9900L, CallbackStatus.SUCCESS,
                                "deadbeef")))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value(40002));

        // 回调支付单不存在 → 404 / 40402
        mockMvc.perform(post("/api/v1/payments/callback")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(callbackBody("P-NOT-EXIST", "WX-TX-0002", CHANNEL, 9900L, CallbackStatus.SUCCESS)))
                .andExpect(status().isNotFound())
                .andExpect(jsonPath("$.code").value(40402));

        // 回调金额与支付单不一致 → 409 / 40906
        mockMvc.perform(post("/api/v1/payments/callback")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(callbackBody(paymentId, "WX-TX-0003", CHANNEL, 1L, CallbackStatus.SUCCESS)))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.code").value(40906));

        // 已完成订单不允许取消 → 409 / 40901
        mockMvc.perform(post("/api/v1/payments/callback")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(callbackBody(paymentId, "WX-TX-0004", CHANNEL, 9900L, CallbackStatus.SUCCESS)))
                .andExpect(status().isOk());
        expect(post("/api/v1/orders/" + orderId + "/cancel"), UID, 409, 40901);
    }

    /** 发送请求：断言 HTTP 状态与统一错误码 */
    private void expect(MockHttpServletRequestBuilder builder, String uid, int httpStatus, int code) throws Exception {
        mockMvc.perform(withUid(builder, uid))
                .andExpect(status().is(httpStatus))
                .andExpect(jsonPath("$.code").value(code));
    }

    /** 发送请求：断言成功（HTTP 200 + code=0），返回响应体 */
    private String ok(MockHttpServletRequestBuilder builder, String uid) throws Exception {
        MvcResult result = mockMvc.perform(withUid(builder, uid))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andReturn();
        return result.getResponse().getContentAsString(StandardCharsets.UTF_8);
    }

    private MockHttpServletRequestBuilder withUid(MockHttpServletRequestBuilder builder, String uid) {
        if (uid != null) {
            builder.header(UID_HEADER, uid);
        }
        return builder.contentType(MediaType.APPLICATION_JSON);
    }

    /** 构造带正确签名的支付回调报文 */
    private String callbackBody(String paymentId, String transactionId, int paymentChannel, long amount,
                               CallbackStatus tradeStatus) {
        PaymentCallbackRequest request = new PaymentCallbackRequest(paymentId, transactionId, paymentChannel,
                tradeStatus, amount, "");
        return callbackBody(paymentId, transactionId, paymentChannel, amount, tradeStatus,
                paymentSigner.sign(request));
    }

    /** 构造指定签名的支付回调报文（用于验签失败场景） */
    private String callbackBody(String paymentId, String transactionId, int paymentChannel, long amount,
                               CallbackStatus tradeStatus, String sign) {
        return """
                {"paymentId": "%s", "transactionId": "%s", "paymentChannel": %d,
                 "tradeStatus": "%s", "amount": %d, "sign": "%s"}
                """.formatted(paymentId, transactionId, paymentChannel, tradeStatus.name(), amount, sign);
    }

    private String orderBody(String skuId, int quantity) {
        return """
                {"items": [{"skuId": "%s", "quantity": %d}]}
                """.formatted(skuId, quantity);
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
        dto.setUid(OTHER_UID);
        dto.setName("HTTP商品");
        dto.setProductType(1);
        dto.setStatus(1);
        return dto;
    }

    private String read(String body, String jsonPath) {
        return JsonPath.read(body, jsonPath);
    }

    private int readInt(String body, String jsonPath) {
        Number value = JsonPath.read(body, jsonPath);
        return value.intValue();
    }

    private long readLong(String body, String jsonPath) {
        Number value = JsonPath.read(body, jsonPath);
        return value.longValue();
    }

    private List<Map<String, Object>> readList(String body, String jsonPath) {
        return JsonPath.read(body, jsonPath);
    }
}
