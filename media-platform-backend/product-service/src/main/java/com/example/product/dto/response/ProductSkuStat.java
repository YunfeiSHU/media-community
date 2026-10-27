package com.example.product.dto.response;

import lombok.Data;

/**
 * 商品下 SKU 的聚合结果（批量查询用，避免列表页 N+1）。
 *
 * <p>用可变 POJO 而不是 record：MyBatis 对 record 需要构造器参数名与列名严格对应，
 * 这里列名带下划线别名（productId / minPrice / skuCount），用 setter 映射更稳。
 */
@Data
public class ProductSkuStat {

    private Long productId;

    /** 最低价（分）；商品下没有可用 SKU 时为 null */
    private Long minPrice;

    /** 未删除的 SKU 数量 */
    private Long skuCount;
}
