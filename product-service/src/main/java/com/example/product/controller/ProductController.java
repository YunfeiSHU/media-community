package com.example.product.controller;

import com.example.product.common.ApiResponse;
import com.example.product.dto.request.CreateProductRequest;
import com.example.product.dto.request.CreateSkuRequest;
import com.example.product.dto.request.UpdateProductRequest;
import com.example.product.dto.response.ProductDetailVO;
import com.example.product.dto.response.SkuVO;
import com.example.product.service.ProductService;
import com.example.product.service.SkuService;
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
 */
@RestController
@RequestMapping("/api/v1/products")
@RequiredArgsConstructor
public class ProductController {

    private final ProductService productService;

    private final SkuService skuService;

    /** 创建商品 POST /api/v1/products */
    @PostMapping
    public ApiResponse<ProductDetailVO> createProduct(@Valid @RequestBody CreateProductRequest request) {
        return ApiResponse.success(productService.createProduct(request));
    }

    /** 商品详情 GET /api/v1/products/{id} */
    @GetMapping("/{id}")
    public ApiResponse<ProductDetailVO> getProduct(@PathVariable("id") String id) {
        return ApiResponse.success(productService.getProduct(id));
    }

    /** 修改商品 PUT /api/v1/products/{id} */
    @PutMapping("/{id}")
    public ApiResponse<ProductDetailVO> updateProduct(@PathVariable("id") String id,
                                                      @Valid @RequestBody UpdateProductRequest request) {
        return ApiResponse.success(productService.updateProduct(id, request));
    }

    /** 删除商品 DELETE /api/v1/products/{id} */
    @DeleteMapping("/{id}")
    public ApiResponse<Void> deleteProduct(@PathVariable("id") String id) {
        productService.deleteProduct(id);
        return ApiResponse.success();
    }

    /** 上架 POST /api/v1/products/{id}/on-shelf */
    @PostMapping("/{id}/on-shelf")
    public ApiResponse<ProductDetailVO> onShelf(@PathVariable("id") String id) {
        return ApiResponse.success(productService.onShelf(id));
    }

    /** 下架 POST /api/v1/products/{id}/off-shelf */
    @PostMapping("/{id}/off-shelf")
    public ApiResponse<ProductDetailVO> offShelf(@PathVariable("id") String id) {
        return ApiResponse.success(productService.offShelf(id));
    }

    /** 创建 SKU POST /api/v1/products/{id}/skus */
    @PostMapping("/{id}/skus")
    public ApiResponse<SkuVO> createSku(@PathVariable("id") String id,
                                        @Valid @RequestBody CreateSkuRequest request) {
        return ApiResponse.success(skuService.createSku(id, request));
    }
}
