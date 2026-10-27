package com.example.product.dto.request;

import jakarta.validation.constraints.Size;

/**
 * 修改商品请求体：字段为 null 表示不修改。
 */
public record UpdateProductRequest(

        @Size(max = 128, message = "商品名称长度不能超过128")
        String name,

        @Size(max = 500, message = "商品描述长度不能超过500")
        String description,

        @Size(max = 512, message = "商品封面地址长度不能超过512")
        String coverUrl,

        Integer productType
) {

    public boolean isEmpty() {
        return name == null && description == null && coverUrl == null && productType == null;
    }
}
