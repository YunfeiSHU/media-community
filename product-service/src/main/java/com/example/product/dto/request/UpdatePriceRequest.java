package com.example.product.dto.request;

import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;

/**
 * 修改价格请求体。
 */
public record UpdatePriceRequest(

        /** 售价，单位：分 */
        @NotNull(message = "价格不能为空")
        @Min(value = 0, message = "价格不能小于0")
        @Max(value = 99999999999L, message = "价格超出允许范围")
        Long price
) {
}
