package com.example.order.dto.request;

import jakarta.validation.Valid;
import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotEmpty;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import java.util.List;

/**
 * 创建订单请求体。
 *
 * <p>商品名称 / 单价等一律由服务端通过 RPC 查询后快照，<b>不接收客户端传入的价格</b>，避免篡改。
 */
public record CreateOrderRequest(

        @NotEmpty(message = "订单商品不能为空")
        @Size(max = 50, message = "单个订单最多包含50个SKU")
        @Valid
        List<OrderItemRequest> items
) {

    /** 订单中的一个 SKU 及其购买数量 */
    public record OrderItemRequest(

            @NotBlank(message = "SKU编号不能为空")
            @Size(max = 32, message = "SKU编号长度不能超过32")
            String skuId,

            @NotNull(message = "购买数量不能为空")
            @Min(value = 1, message = "购买数量至少为1")
            @Max(value = 999, message = "单个SKU购买数量不能超过999")
            Integer quantity
    ) {
    }
}
