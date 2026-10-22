package com.example.product.config;

import org.apache.dubbo.config.spring.context.annotation.EnableDubbo;
import org.springframework.context.annotation.Configuration;

/**
 * Dubbo 配置：扫描 {@code com.example.product.api} 下的 RPC 接口与实现。
 */
@Configuration
@EnableDubbo(scanBasePackages = "com.example.product.api")
public class DubboConfig {
}
