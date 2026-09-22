package com.example.product.dto.request;

import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

/**
 * 创建 SKU 请求体。
 */
public record CreateSkuRequest(

        @NotBlank(message = "SKU名称不能为空")
        @Size(max = 128, message = "SKU名称长度不能超过128")
        String name,

        @NotNull(message = "售价不能为空")
        @Min(value = 0, message = "售价不能小于0")
        @Max(value = 99999999999L, message = "售价超出允许范围")
        Long price,

        /** 初始可用库存，不传按 0 处理 */
        @Min(value = 0, message = "初始库存不能小于0")
        Integer initialStock
) {

    public int initialStockOrDefault() {
        return initialStock == null ? 0 : initialStock;
    }
}
