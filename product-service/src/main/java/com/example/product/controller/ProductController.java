package com.example.product.controller;

import com.example.product.common.ApiResponse;
import com.example.product.common.CurrentUser;
import com.example.product.dto.request.CreateProductRequest;
import com.example.product.dto.request.CreateSkuRequest;
import com.example.product.dto.request.UpdateProductRequest;
import com.example.product.dto.response.ProductDetailVO;
import com.example.product.dto.response.SkuVO;
import com.example.product.service.ProductService;
import com.example.product.service.SkuService;
import com.example.product.support.ProductOwnership;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * 商品相关 HTTP 接口。
 *
 * <p>写接口（创建/修改/删除/上下架/建 SKU）在鉴权之外还做<b>归属校验</b>：
 * 只有商品创建者（{@code product.uid} == 网关注入的 {@code X-User-Id}）才能操作，
 * 他人商品一律 403 / 40301（见 {@link ProductOwnership}）。只读接口保持公开。
 */
@RestController
@RequestMapping("/api/v1/products")
@RequiredArgsConstructor
public class ProductController {

    private final ProductService productService;

    private final SkuService skuService;

    private final ProductOwnership productOwnership;

    /** 创建商品 POST /api/v1/products（创建者取网关注入的 X-User-Id） */
    @PostMapping
    public ApiResponse<ProductDetailVO> createProduct(@Valid @RequestBody CreateProductRequest request,
                                                      HttpServletRequest httpRequest) {
        return ApiResponse.success(productService.createProduct(CurrentUser.uid(httpRequest), request));
    }

    /** 商品详情 GET /api/v1/products/{id} */
    @GetMapping("/{id}")
    public ApiResponse<ProductDetailVO> getProduct(@PathVariable("id") String id) {
        return ApiResponse.success(productService.getProduct(id));
    }

    /** 修改商品 PUT /api/v1/products/{id}（仅创建者） */
    @PutMapping("/{id}")
    public ApiResponse<ProductDetailVO> updateProduct(@PathVariable("id") String id,
                                                      @Valid @RequestBody UpdateProductRequest request,
                                                      HttpServletRequest httpRequest) {
        productOwnership.requireProductOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(productService.updateProduct(id, request));
    }

    /** 删除商品 DELETE /api/v1/products/{id}（仅创建者） */
    @DeleteMapping("/{id}")
    public ApiResponse<Void> deleteProduct(@PathVariable("id") String id,
                                           HttpServletRequest httpRequest) {
        productOwnership.requireProductOwner(id, CurrentUser.uid(httpRequest));
        productService.deleteProduct(id);
        return ApiResponse.success();
    }

    /** 上架 POST /api/v1/products/{id}/on-shelf（仅创建者） */
    @PostMapping("/{id}/on-shelf")
    public ApiResponse<ProductDetailVO> onShelf(@PathVariable("id") String id,
                                                HttpServletRequest httpRequest) {
        productOwnership.requireProductOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(productService.onShelf(id));
    }

    /** 下架 POST /api/v1/products/{id}/off-shelf（仅创建者） */
    @PostMapping("/{id}/off-shelf")
    public ApiResponse<ProductDetailVO> offShelf(@PathVariable("id") String id,
                                                 HttpServletRequest httpRequest) {
        productOwnership.requireProductOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(productService.offShelf(id));
    }

    /** 创建 SKU POST /api/v1/products/{id}/skus（仅商品创建者） */
    @PostMapping("/{id}/skus")
    public ApiResponse<SkuVO> createSku(@PathVariable("id") String id,
                                        @Valid @RequestBody CreateSkuRequest request,
                                        HttpServletRequest httpRequest) {
        productOwnership.requireProductOwner(id, CurrentUser.uid(httpRequest));
        return ApiResponse.success(skuService.createSku(id, request));
    }
}
