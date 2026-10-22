package com.example.order.controller;

import com.example.order.common.ApiResponse;
import com.example.order.common.CurrentUser;
import com.example.order.common.PageVO;
import com.example.order.dto.request.CreateOrderRequest;
import com.example.order.dto.request.CreatePaymentRequest;
import com.example.order.dto.response.OrderVO;
import com.example.order.dto.response.PaymentVO;
import com.example.order.service.OrderService;
import com.example.order.service.PaymentService;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * 订单相关 HTTP 接口。
 *
 * <p>登录态由网关保证：JWT 校验通过后，网关把 claim {@code sub} 写入 {@code X-User-Id}、
 * 把 {@code role} 写入 {@code X-User-Role}，服务只从这两个头取当前用户，
 * 一律不接受请求体 / 查询参数里的用户标识与角色。
 */
@RestController
@RequestMapping("/api/v1/orders")
@RequiredArgsConstructor
public class OrderController {

    private final OrderService orderService;

    private final PaymentService paymentService;

    /** 创建订单 POST /api/v1/orders */
    @PostMapping
    public ApiResponse<OrderVO> createOrder(@Valid @RequestBody CreateOrderRequest request,
                                            HttpServletRequest httpRequest) {
        return ApiResponse.success(orderService.createOrder(CurrentUser.uid(httpRequest), request));
    }

    /** 查询我的订单 GET /api/v1/orders?status=&page=&size= */
    @GetMapping
    public ApiResponse<PageVO<OrderVO>> listMyOrders(
            @RequestParam(value = "status", required = false) Integer status,
            @RequestParam(value = "page", defaultValue = "1") int page,
            @RequestParam(value = "size", defaultValue = "10") int size,
            HttpServletRequest httpRequest) {
        return ApiResponse.success(orderService.listMyOrders(CurrentUser.uid(httpRequest), status, page, size));
    }

    /** 查询订单详情 GET /api/v1/orders/{id} */
    @GetMapping("/{id}")
    public ApiResponse<OrderVO> getOrder(@PathVariable("id") String id, HttpServletRequest httpRequest) {
        return ApiResponse.success(orderService.getOrder(CurrentUser.uid(httpRequest), id));
    }

    /** 取消订单 POST /api/v1/orders/{id}/cancel */
    @PostMapping("/{id}/cancel")
    public ApiResponse<OrderVO> cancelOrder(@PathVariable("id") String id, HttpServletRequest httpRequest) {
        return ApiResponse.success(orderService.cancelOrder(CurrentUser.uid(httpRequest), id));
    }

    /** 创建支付单 POST /api/v1/orders/{id}/pay */
    @PostMapping("/{id}/pay")
    public ApiResponse<PaymentVO> pay(@PathVariable("id") String id,
                                      @Valid @RequestBody CreatePaymentRequest request,
                                      HttpServletRequest httpRequest) {
        return ApiResponse.success(paymentService.createPayment(CurrentUser.uid(httpRequest), id, request));
    }
}
