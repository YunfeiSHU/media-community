package com.example.product.mapper;

import com.example.product.domain.entity.Product;
import org.apache.ibatis.annotations.Param;

/**
 * 商品表 Mapper。SQL 见 {@code resources/mapper/ProductMapper.xml}。
 */
public interface ProductMapper {

    /** 新增商品，回填自增主键 */
    int insert(Product product);

    Product selectById(@Param("id") Long id);

    Product selectByProductId(@Param("productId") String productId);

    /** 按非空字段动态更新 */
    int update(Product product);

    /**
     * 状态流转（带乐观校验）。
     *
     * @param expectedStatus 期望的当前状态，为 null 表示不校验
     * @return 影响行数，0 表示状态已被并发修改
     */
    int updateStatus(@Param("id") Long id,
                     @Param("status") int status,
                     @Param("expectedStatus") Integer expectedStatus);
}
