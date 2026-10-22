package com.example.product.service;

import com.example.product.dto.request.CreateSkuRequest;
import com.example.product.dto.request.UpdateSkuRequest;
import com.example.product.dto.response.SkuVO;

/**
 * SKU 域服务：创建 / 查询 / 修改 / 删除。
 */
public interface SkuService {

    /** 在指定商品下创建 SKU，同时初始化库存记录 */
    SkuVO createSku(String productId, CreateSkuRequest request);

    /** SKU 详情（含库存快照），{@code id} 支持内部ID或业务编号 */
    SkuVO getSku(String id);

    /** 修改 SKU 名称与状态 */
    SkuVO updateSku(String id, UpdateSkuRequest request);

    /** 删除 SKU（逻辑删除） */
    void deleteSku(String id);
}
