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

        /**
         * product-service 的 Dubbo 直连地址，如 {@code dubbo://127.0.0.1:20880}。
         *
         * <p>该值会直接注入 {@code @DubboReference(url = ...)}，而 Dubbo 中 url 的优先级高于注册中心，
         * 所以要改成“走注册中心发现”时必须<b>删掉配置项</b>（让占位符回落到默认值不可行），
         * 而不是把它置为空串：空串会让 Dubbo 拿不到合法 url 而启动失败。
         */
        private String productUrl = "dubbo://127.0.0.1:20880";
    }

    /** 支付相关配置 */
    @Data
    public static class Payment {

        /**
         * 支付回调验签密钥（HmacSHA256）。
         *
         * <p>默认空串：此时 {@code PaymentSigner} 跳过验签并列日志告警，仅供本地联调；
         * 生产必须配置（回调接口免登录，验签是它唯一的鉴权手段）。
         */
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
