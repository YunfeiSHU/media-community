package com.example.order.controller;

import com.example.order.common.ApiResponse;
import com.example.order.common.CurrentUser;
import com.example.order.dto.response.PaymentVO;
import com.example.order.service.PaymentService;
import jakarta.servlet.http.HttpServletRequest;
import lombok.RequiredArgsConstructor;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * 模拟支付入口（仅本地/演示）。
 *
 * <p>真实支付必须由渠道异步回调驱动，而本仓库没有接真实渠道。没有这个入口时，
 * 用户点完“支付”订单会永远停在待支付——商城的 UI 闭环是断的。
 *
 * <p>安全边界：
 * <ul>
 *     <li>必须登录，且只能操作**自己的**订单（{@code OrderLookup#requireMine}）；</li>
 *     <li>不做验签，因为它不模拟“渠道”，只模拟“已确认到账”这一步；</li>
 *     <li>生产通过 {@code MOCK_PAY_ENABLED=false} 关闭，或干脆不部署该路由
 *         （与 live 的 {@code /dev/**} 同思路）。</li>
 * </ul>
 */
@RestController
@RequestMapping("/api/v1/orders")
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "order.mock-pay", name = "enabled", havingValue = "true", matchIfMissing = true)
public class MockPayController {

    private final PaymentService paymentService;

    /** POST /api/v1/orders/{id}/mock-pay */
    @PostMapping("/{id}/mock-pay")
    public ApiResponse<PaymentVO> mockPay(@PathVariable("id") String id, HttpServletRequest httpRequest) {
        return ApiResponse.success(paymentService.mockPay(CurrentUser.uid(httpRequest), id));
    }
}
