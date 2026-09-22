package com.example.order.common;

import java.util.List;

/**
 * 统一分页结构。
 *
 * @param page 当前页码，从 1 开始
 * @param size 每页条数
 * @param total 满足条件的总条数
 * @param list 当前页数据
 */
public record PageVO<T>(int page, int size, long total, List<T> list) {

    public static <T> PageVO<T> of(int page, int size, long total, List<T> list) {
        return new PageVO<>(page, size, total, list);
    }
}
