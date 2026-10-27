package com.example.product.service;

import com.example.product.common.PageResult;
import com.example.product.dto.request.CreateProductRequest;
import com.example.product.dto.request.UpdateProductRequest;
import com.example.product.dto.response.ProductDetailVO;
import com.example.product.dto.response.ProductVO;

/**
 * 商品域服务：创建 / 详情 / 修改 / 删除 / 上下架。
 */
public interface ProductService {

    /**
     * 创建商品，初始状态为「下架」。
     *
     * @param uid 当前登录用户 uid，由网关注入的 {@code X-User-Id} 得到，不能来自请求体
     */
    ProductDetailVO createProduct(String uid, CreateProductRequest request);

    /** 商品详情（含 SKU 与库存快照），{@code id} 支持内部ID或业务编号 */
    ProductDetailVO getProduct(String id);

    /**
     * 商品分页列表（商城首页）。
     *
     * @param keyword 按名称模糊匹配，空表示不过滤
     * @param status  状态过滤；公开接口固定传 {@code ON_SHELF}
     */
    PageResult<ProductVO> listProducts(String keyword, Integer status, int page, int pageSize);

    /** 我的商品（含草稿/下架），按创建者 uid 过滤。 */
    PageResult<ProductVO> listMyProducts(String uid, int page, int pageSize);

    /** 修改商品基础信息（name / description / coverUrl / productType） */
    ProductDetailVO updateProduct(String id, UpdateProductRequest request);

    /** 删除商品（逻辑删除，同时级联逻辑删除其 SKU） */
    void deleteProduct(String id);

    /** 上架：要求商品下至少有一个启用中的 SKU */
    ProductDetailVO onShelf(String id);

    /** 下架 */
    ProductDetailVO offShelf(String id);
}
