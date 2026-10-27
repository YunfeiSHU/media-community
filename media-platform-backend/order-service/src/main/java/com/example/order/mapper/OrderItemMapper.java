package com.example.order.mapper;

import com.example.order.domain.entity.OrderItem;
import java.util.Collection;
import java.util.List;
import org.apache.ibatis.annotations.Param;

/**
 * 订单明细表 Mapper。SQL 见 {@code resources/mapper/OrderItemMapper.xml}。
 */
public interface OrderItemMapper {

    /** 批量新增订单明细，回填自增主键 */
    int batchInsert(@Param("items") List<OrderItem> items);

    /** 查询单个订单的明细 */
    List<OrderItem> selectByOrderId(@Param("orderId") Long orderId);

    /** 批量查询多个订单的明细，避免列表接口 N+1 查询 */
    List<OrderItem> selectByOrderIds(@Param("orderIds") Collection<Long> orderIds);
}
