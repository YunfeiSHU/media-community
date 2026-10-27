package com.example.product.dto.response;

import java.time.LocalDateTime;

/**
 * 当前价格视图。
 *
 * @param price 售价，单位：分
 */
public record PriceVO(String skuId, Long price, String currency, LocalDateTime updatedAt) {

    public static final String CURRENCY_CNY = "CNY";
}
