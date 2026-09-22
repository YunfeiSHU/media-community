package com.example.order.mapper;

import com.example.order.domain.entity.Order;
import java.time.LocalDateTime;
import java.util.List;
import org.apache.ibatis.annotations.Param;

/**
 * 订单表 Mapper。SQL 见 {@code resources/mapper/OrderMapper.xml}。
 *
 * <p>状态流转统一使用「条件 UPDATE + 影响行数」做乐观校验，避免先查后改的并发窗口。
 */
public interface OrderMapper {

    /** 新增订单，回填自增主键 */
    int insert(Order order);

    Order selectById(@Param("id") Long id);

    Order selectByOrderId(@Param("orderId") String orderId);

    /** 查询「我的订单」详情：非本人订单查不到 */
    Order selectByOrderIdAndUid(@Param("orderId") String orderId, @Param("uid") String uid);

    /**
     * 状态流转：WAIT_PAY(0) → PAID(1)。
     *
     * @return 影响行数，0 表示订单已不是待支付状态（并发下已被取消 / 已支付）
     */
    int markPaid(@Param("id") Long id, @Param("paidAt") LocalDateTime paidAt);

    /**
     * 状态流转：PAID(1) → COMPLETED(2)。
     *
     * @return 影响行数，0 表示订单状态已变更
     */
    int markCompleted(@Param("id") Long id, @Param("completedAt") LocalDateTime completedAt);

    /**
     * 状态流转：WAIT_PAY(0) → CANCELLED(3)。
     *
     * @return 影响行数，0 表示订单状态已变更（并发下已被支付 / 已取消）
     */
    int markCancelled(@Param("id") Long id, @Param("cancelledAt") LocalDateTime cancelledAt);

    /** 分页查询某个用户的订单，按创建时间倒序 */
    List<Order> selectByUid(@Param("uid") String uid,
                            @Param("status") Integer status,
                            @Param("offset") int offset,
                            @Param("limit") int limit);

    /** 统计某个用户满足条件的订单总数 */
    long countByUid(@Param("uid") String uid, @Param("status") Integer status);

    /** 扫描已超时仍未支付的订单（供定时任务使用） */
    List<Order> selectExpired(@Param("now") LocalDateTime now, @Param("limit") int limit);

    /** 扫描已支付但权益尚未发放完成的订单（供补偿任务使用） */
    List<Order> selectPaidBefore(@Param("paidBefore") LocalDateTime paidBefore, @Param("limit") int limit);
}
