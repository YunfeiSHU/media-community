package com.example.product.dto.request;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;

/**
 * 创建商品请求体。
 *
 * <p><b>不包含创建者 UID</b>：创建者一律取网关校验 JWT 后注入的 {@code X-User-Id}
 * （即 JWT 的 {@code sub}），不接受客户端自报，见
 * {@link com.example.product.common.CurrentUser}。
 */
public record CreateProductRequest(

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
