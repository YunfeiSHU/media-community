package com.example.product.dto.request;

import jakarta.validation.constraints.Max;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.Size;

/**
 * 修改 SKU 请求体：只允许改名称与状态（0-禁用 1-启用），价格走价格接口。
 */
public record UpdateSkuRequest(

        @Size(max = 128, message = "SKU名称长度不能超过128")
        String name,

        @Min(value = 0, message = "SKU状态只能是0或1")
        @Max(value = 1, message = "SKU状态只能是0或1")
        Integer status
) {

    public boolean isEmpty() {
        return name == null && status == null;
    }
}
