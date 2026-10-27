package com.example.product.service;

import com.example.product.dto.request.UpdatePriceRequest;
import com.example.product.dto.response.PriceVO;

/**
 * 价格域服务（当前价格即 SKU 售价）。
 */
public interface PriceService {

    /** 查询 SKU 当前价格 */
    PriceVO getCurrentPrice(String skuId);

    /** 修改 SKU 价格 */
    PriceVO updatePrice(String skuId, UpdatePriceRequest request);
}
