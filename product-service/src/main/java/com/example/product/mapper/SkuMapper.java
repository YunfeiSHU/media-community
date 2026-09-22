package com.example.product.mapper;

import com.example.product.domain.entity.Sku;
import java.util.List;
import org.apache.ibatis.annotations.Param;

/**
 * SKU 表 Mapper。SQL 见 {@code resources/mapper/SkuMapper.xml}。
 */
public interface SkuMapper {

    int insert(Sku sku);

    Sku selectById(@Param("id") Long id);

    Sku selectBySkuId(@Param("skuId") String skuId);

    /** 查询商品下未删除的 SKU */
    List<Sku> selectByProductId(@Param("productId") Long productId);

    int countByProductIdAndStatus(@Param("productId") Long productId,
                                  @Param("status") int status);

    /** 按非空字段动态更新（name / price / status） */
    int update(Sku sku);

    int updateStatusById(@Param("id") Long id, @Param("status") int status);

    /** 批量逻辑删除商品下的 SKU */
    int updateStatusByProductId(@Param("productId") Long productId,
                                @Param("status") int status);

    int updatePriceById(@Param("id") Long id, @Param("price") long price);
}
