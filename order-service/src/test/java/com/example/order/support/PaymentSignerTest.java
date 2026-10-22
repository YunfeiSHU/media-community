package com.example.order.support;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;
import com.example.order.config.OrderProperties;
import com.example.order.domain.enums.CallbackStatus;
import com.example.order.dto.request.PaymentCallbackRequest;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

/**
 * 支付回调验签的行为约定（纯单元测试，不启 Spring 上下文）。
 *
 * <p>重点覆盖两件事：
 * <ol>
 *     <li>配置了密钥时，签名错误 / 缺失一律拒绝（40002），并且是常量时间比较；</li>
 *     <li>未配置密钥时按约定跳过验签（本地联调），而不是“偷偷用一个固定默认密钥假装验签”——
 *     那正是把 {@code callback-secret} 写成公开默认值时的真实风险。</li>
 * </ol>
 */
class PaymentSignerTest {

    private static final String SECRET = "unit-test-secret";

    private static PaymentCallbackRequest request(String sign) {
        return new PaymentCallbackRequest("P-1", "TX-1", 2, CallbackStatus.SUCCESS, 9900L, sign);
    }

    private static PaymentSigner signerWith(String secret) {
        OrderProperties properties = new OrderProperties();
        properties.getPayment().setCallbackSecret(secret);
        return new PaymentSigner(properties);
    }

    @Test
    @DisplayName("配置了密钥：正确签名通过，enabled() 为 true")
    void acceptsValidSignature() {
        PaymentSigner signer = signerWith(SECRET);
        assertThat(signer.enabled()).isTrue();

        PaymentCallbackRequest unsigned = request("");
        String sign = signer.sign(unsigned);
        assertThatCode(() -> signer.verify(request(sign))).doesNotThrowAnyException();
    }

    @Test
    @DisplayName("配置了密钥：签名错误 / 缺失 → 40002")
    void rejectsInvalidSignature() {
        PaymentSigner signer = signerWith(SECRET);

        assertThatThrownBy(() -> signer.verify(request("deadbeef")))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.INVALID_PAYMENT_SIGN);

        // null 签名不能被当成“合法跳过”
        assertThatThrownBy(() -> signer.verify(request(null)))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.INVALID_PAYMENT_SIGN);
    }

    @Test
    @DisplayName("签名比较忽略大小写与首尾空格（渠道回传常见处理）")
    void normalizesSignBeforeComparing() {
        PaymentSigner signer = signerWith(SECRET);
        String sign = signer.sign(request(""));

        assertThatCode(() -> signer.verify(request("  " + sign.toUpperCase(java.util.Locale.ROOT) + "  ")))
                .doesNotThrowAnyException();
    }

    @Test
    @DisplayName("未配置密钥：enabled() 为 false 且跳过验签（仅本地联调，启动会 WARN）")
    void skipsVerificationWhenSecretMissing() {
        PaymentSigner signer = signerWith("");
        assertThat(signer.enabled()).isFalse();
        assertThatCode(() -> signer.verify(request(""))).doesNotThrowAnyException();

        PaymentSigner nullSecret = signerWith(null);
        assertThat(nullSecret.enabled()).isFalse();
        assertThatCode(() -> nullSecret.verify(request(null))).doesNotThrowAnyException();
    }
}
