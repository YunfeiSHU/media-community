package com.example.order.support;

import com.example.order.common.BizException;
import com.example.order.common.ErrorCode;
import com.example.order.config.OrderProperties;
import com.example.order.dto.request.PaymentCallbackRequest;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.Locale;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

/**
 * 支付回调验签。
 *
 * <p>算法：{@code HmacSHA256(secret, canonical)}，{@code canonical} 为按字段名升序拼接的
 * {@code key=value&...} 串（见 {@link PaymentCallbackRequest} 的类注释）。
 * 密钥由 {@code order.payment.callback-secret} 配置；未配置时跳过验签（仅限本地联调，生产必须配置）。
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class PaymentSigner {

    private static final String ALGORITHM = "HmacSHA256";

    private final OrderProperties properties;

    /** 是否启用了验签 */
    public boolean enabled() {
        String secret = properties.getPayment().getCallbackSecret();
        return secret != null && !secret.isBlank();
    }

    /** 计算签名（渠道侧需使用同一算法与密钥；本地联调可用它构造回调请求） */
    public String sign(PaymentCallbackRequest request) {
        return hmac(canonical(request));
    }

    /**
     * 校验签名。
     *
     * @throws BizException 验签失败时抛出 40002
     */
    public void verify(PaymentCallbackRequest request) {
        if (!enabled()) {
            log.warn("未配置 order.payment.callback-secret，已跳过支付回调验签（仅限本地联调）: paymentId={}",
                    request.paymentId());
            return;
        }
        String expected = sign(request);
        String actual = request.sign() == null ? "" : request.sign().trim().toLowerCase(Locale.ROOT);
        // 常量时间比较，避免时序侧信道
        if (!MessageDigest.isEqual(expected.getBytes(StandardCharsets.UTF_8), actual.getBytes(StandardCharsets.UTF_8))) {
            throw new BizException(ErrorCode.INVALID_PAYMENT_SIGN, "支付回调签名校验失败: paymentId=" + request.paymentId());
        }
    }

    private String canonical(PaymentCallbackRequest request) {
        return "amount=" + request.amount()
                + "&paymentChannel=" + request.paymentChannel()
                + "&paymentId=" + request.paymentId()
                + "&tradeStatus=" + request.tradeStatus().name()
                + "&transactionId=" + request.transactionId();
    }

    private String hmac(String content) {
        try {
            Mac mac = Mac.getInstance(ALGORITHM);
            mac.init(new SecretKeySpec(properties.getPayment().getCallbackSecret().getBytes(StandardCharsets.UTF_8),
                    ALGORITHM));
            return HexFormat.of().formatHex(mac.doFinal(content.getBytes(StandardCharsets.UTF_8)));
        } catch (GeneralSecurityException ex) {
            throw new IllegalStateException("计算支付回调签名失败", ex);
        }
    }
}
