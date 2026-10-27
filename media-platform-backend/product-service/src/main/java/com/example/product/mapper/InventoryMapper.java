package com.example.product.mapper;

import com.example.product.domain.entity.Inventory;
import java.util.Collection;
import java.util.List;
import org.apache.ibatis.annotations.Param;

/**
 * 库存表 Mapper。
 *
 * <p>库存变更全部使用「条件 UPDATE + 影响行数判断」实现，把并发控制交给数据库行锁，
 * 避免先查后改（check-then-act）带来的超卖问题。
 */
public interface InventoryMapper {

    int insert(Inventory inventory);

    Inventory selectBySkuId(@Param("skuId") Long skuId);

    List<Inventory> selectBySkuIds(@Param("skuIds") Collection<Long> skuIds);

    /** 增加可用库存 */
    int increaseAvailable(@Param("skuId") Long skuId, @Param("quantity") int quantity);

    /** 扣减可用库存并同步锁定（订单预占），可用库存不足时影响行数为 0 */
    int reserve(@Param("skuId") Long skuId, @Param("quantity") int quantity);

    /** 确认扣减：锁定库存转成真实消耗，锁定库存不足时影响行数为 0 */
    int confirm(@Param("skuId") Long skuId, @Param("quantity") int quantity);

    /** 释放锁定库存：锁定库存回到可用库存，锁定库存不足时影响行数为 0 */
    int release(@Param("skuId") Long skuId, @Param("quantity") int quantity);
}
