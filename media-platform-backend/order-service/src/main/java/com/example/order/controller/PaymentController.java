package com.example.order.controller;

import com.example.order.common.ApiResponse;
import com.example.order.dto.request.PaymentCallbackRequest;
import com.example.order.service.PaymentService;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * 支付相关 HTTP 接口。
 */
@RestController
@RequestMapping("/api/v1/payments")
@RequiredArgsConstructor
public class PaymentController {

    private final PaymentService paymentService;

    /**
     * 支付渠道异步回调 POST /api/v1/payments/callback
     *
     * <p>免登录接口（渠道服务器无法携带用户令牌），需加入网关白名单；身份由支付单反查得到。
     * 真实性靠报文签名保证，见 {@code PaymentSigner}。
     */
    @PostMapping("/callback")
    public ApiResponse<Void> callback(@Valid @RequestBody PaymentCallbackRequest request) {
        paymentService.handleCallback(request);
        return ApiResponse.success();
    }
}
