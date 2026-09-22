package com.example.order.config;

import lombok.Data;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.stereotype.Component;

/**
 * 订单中心可配置参数（前缀 {@code order}）。
 */
@Data
@Component
@ConfigurationProperties(prefix = "order")
public class OrderProperties {

    /** 待支付订单的超时时间（分钟） */
    private int expireMinutes = 30;

    private Rpc rpc = new Rpc();

    private Payment payment = new Payment();

    private Task task = new Task();

    /** 下游 RPC 配置 */
    @Data
    public static class Rpc {

        /** product-service 的 Dubbo 直连地址，如 {@code dubbo://127.0.0.1:20880}；为空则依赖注册中心发现 */
        private String productUrl = "dubbo://127.0.0.1:20880";
    }

    /** 支付相关配置 */
    @Data
    public static class Payment {

        /** 支付回调验签密钥（HmacSHA256），为空表示跳过验签（仅本地联调使用） */
        private String callbackSecret = "";
    }

    /** 定时任务配置 */
    @Data
    public static class Task {

        /** 定时任务总开关 */
        private boolean enabled = true;

        /** 单轮扫描处理的订单数量上限 */
        private int batchSize = 100;

        /** 扫描间隔（毫秒） */
        private long scanIntervalMs = 60_000L;

        /** 启动后首次执行的延迟（毫秒），避免与启动期其它初始化抢资源 */
        private long initialDelayMs = 30_000L;
    }
}
