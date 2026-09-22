package com.example.product.dto.request;

import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;

/**
 * 库存变更请求体（扣减 / 释放 / 确认 / 增加）。
 */
public record StockChangeRequest(

        @NotNull(message = "库存数量不能为空")
        @Min(value = 1, message = "库存数量必须大于0")
        Integer quantity
) {
}
