package com.example.product.dto.request;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

/**
 * 创建商品请求体。
 */
public record CreateProductRequest(

        @NotBlank(message = "创建者UID不能为空")
        @Size(max = 32, message = "创建者UID长度不能超过32")
        String uid,

        @NotBlank(message = "商品名称不能为空")
        @Size(max = 128, message = "商品名称长度不能超过128")
        String name,

        @Size(max = 500, message = "商品描述长度不能超过500")
        String description,

        @Size(max = 512, message = "商品封面地址长度不能超过512")
        String coverUrl,

        @NotNull(message = "商品类型不能为空")
        Integer productType
) {
}
