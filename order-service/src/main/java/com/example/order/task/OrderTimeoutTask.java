package com.example.order.task;

import com.example.order.config.OrderProperties;
import com.example.order.service.OrderService;
import java.time.LocalDateTime;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * 订单定时任务。
 *
 * <p>两个子任务共用一次「扫描 + 幂等处理」的模式：先批量捞出候选订单编号，再逐个在独立事务里处理，
 * 单个订单失败不影响整批（下轮扫描会自然重试）。
 *
 * <p>测试环境通过 {@code order.task.enabled=false} 关闭。
 */
@Slf4j
@Component
@RequiredArgsConstructor
@ConditionalOnProperty(prefix = "order.task", name = "enabled", havingValue = "true", matchIfMissing = true)
public class OrderTimeoutTask {

    private final OrderService orderService;

    private final OrderProperties properties;

    /**
     * 订单超时取消：WAIT_PAY 且已过 {@code expire_at} 的订单 → CANCELLED，并释放锁定的库存、关闭支付单。
     */
    @Scheduled(fixedDelayString = "${order.task.scan-interval-ms:60000}",
            initialDelayString = "${order.task.initial-delay-ms:30000}")
    public void cancelExpiredOrders() {
        LocalDateTime now = LocalDateTime.now();
        List<String> orderIds = orderService.findExpiredOrderIds(now, properties.getTask().getBatchSize());
        if (orderIds.isEmpty()) {
            log.debug("订单超时取消任务：本轮无待处理订单");
            return;
        }
        int cancelled = 0;
        for (String orderId : orderIds) {
            try {
                if (orderService.cancelExpiredOrder(orderId)) {
                    cancelled++;
                }
            } catch (Exception ex) {
                log.error("订单超时取消失败，等待下轮重试: orderId={}", orderId, ex);
            }
        }
        log.info("订单超时取消任务完成: 扫描={}, 取消={}", orderIds.size(), cancelled);
    }

    /**
     * 权益发放补偿：已支付但未流转到 COMPLETED 的订单，重试确认库存扣减。
     *
     * <p>支付回调里的权益发放是「尽力而为」的（RPC 可能瞬时失败），这里是兜底重试。
     */
    @Scheduled(fixedDelayString = "${order.task.scan-interval-ms:60000}",
            initialDelayString = "${order.task.initial-delay-ms:30000}")
    public void retryCompleteOrders() {
        List<String> orderIds = orderService.findCompletableOrderIds(LocalDateTime.now(),
                properties.getTask().getBatchSize());
        if (orderIds.isEmpty()) {
            log.debug("权益发放补偿任务：本轮无待处理订单");
            return;
        }
        int completed = 0;
        for (String orderId : orderIds) {
            try {
                if (orderService.completePaidOrder(orderId)) {
                    completed++;
                }
            } catch (Exception ex) {
                log.error("权益发放补偿失败，等待下轮重试: orderId={}", orderId, ex);
            }
        }
        log.info("权益发放补偿任务完成: 扫描={}, 完成={}", orderIds.size(), completed);
    }
}
