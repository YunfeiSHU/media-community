package com.example.userservice.common;

import java.util.List;

/**
 * 统一分页结构：{@code {items,total,page,pageSize}}。
 *
 * <p>与 Go 侧、product-service 新接口保持一致；order-service 的
 * {@code PageVO}（{@code {page,size,total,list}}）属于待收敛的存量格式。
 */
public record PageResult<T>(List<T> items, long total, int page, int pageSize) {

    public static <T> PageResult<T> of(List<T> items, long total, int page, int pageSize) {
        return new PageResult<>(items, total, page, pageSize);
    }

    public static <T> PageResult<T> empty(int page, int pageSize) {
        return new PageResult<>(List.of(), 0L, page, pageSize);
    }
}
