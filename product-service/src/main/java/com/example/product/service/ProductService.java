package com.example.product.service;

import com.example.product.dto.request.CreateProductRequest;
import com.example.product.dto.request.UpdateProductRequest;
import com.example.product.dto.response.ProductDetailVO;

/**
 * 商品域服务：创建 / 详情 / 修改 / 删除 / 上下架。
 */
public interface ProductService {

    /** 创建商品，初始状态为「下架」 */
    ProductDetailVO createProduct(CreateProductRequest request);

    /** 商品详情（含 SKU 与库存快照），{@code id} 支持内部ID或业务编号 */
    ProductDetailVO getProduct(String id);

    /** 修改商品基础信息（name / description / coverUrl / productType） */
    ProductDetailVO updateProduct(String id, UpdateProductRequest request);

    /** 删除商品（逻辑删除，同时级联逻辑删除其 SKU） */
    void deleteProduct(String id);

    /** 上架：要求商品下至少有一个启用中的 SKU */
    ProductDetailVO onShelf(String id);

    /** 下架 */
    ProductDetailVO offShelf(String id);
}
