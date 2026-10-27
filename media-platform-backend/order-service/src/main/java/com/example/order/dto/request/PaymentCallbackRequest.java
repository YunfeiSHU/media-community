package com.example.order.dto.request;

import com.example.order.domain.enums.CallbackStatus;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

/**
 * 支付渠道异步回调请求体。
 *
 * <p>该接口不要求登录（渠道服务器无法携带用户令牌），因此 {@code uid} 必须在服务端由支付单反查，
 * 绝不能信任回调报文里的用户标识。
 *
 * <p>验签串（按字段名升序拼接，值为原始字面量）：
 * <pre>
 * amount={amount}&amp;paymentChannel={paymentChannel}&amp;paymentId={paymentId}&amp;tradeStatus={tradeStatus}&amp;transactionId={transactionId}
 * </pre>
 * 签名算法 {@code HmacSHA256}，输出小写十六进制。
 */
public record PaymentCallbackRequest(

        @NotBlank(message = "支付单号不能为空")
        @Size(max = 32, message = "支付单号长度不能超过32")
        String paymentId,

        @NotBlank(message = "渠道流水号不能为空")
        @Size(max = 128, message = "渠道流水号长度不能超过128")
        String transactionId,

        @NotNull(message = "支付渠道不能为空")
        Integer paymentChannel,

        @NotNull(message = "回调结果不能为空")
        CallbackStatus tradeStatus,

        @NotNull(message = "支付金额不能为空")
        @Min(value = 1, message = "支付金额必须大于0")
        Long amount,

        @NotBlank(message = "签名不能为空")
        @Size(max = 128, message = "签名长度不能超过128")
        String sign
) {
}
