package com.example.order.config;

import org.apache.dubbo.config.spring.context.annotation.EnableDubbo;
import org.springframework.context.annotation.Configuration;

/**
 * Dubbo 配置。
 *
 * <p>order-service 是纯消费方：只通过 {@code @DubboReference} 调用 product-service 暴露的
 * {@code ProductRpcService} / {@code StockRpcService}，自身不暴露任何 RPC 服务，因此
 * {@link EnableDubbo} 不需要指定扫描包。
 *
 * <p>注意：Dubbo 3.3 的 spring-boot starter 只自动装配了宽松绑定与 Triple，<b>不会</b>自动开启注解处理，
 * 所以这里的 {@link EnableDubbo} 是 {@code @DubboReference} 能注入的必需条件，不能删。
 * 启动时会有一条 “No class annotated by Dubbo @DubboService ... was found under package
 * [com.example.order.config]” 的 WARN，对本服务属预期行为（确实没有任何 provider 实现）。
 */
@Configuration
@EnableDubbo
public class DubboConfig {
}
