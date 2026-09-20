package com.example.gatewayservice.config;

import org.springframework.boot.context.properties.ConfigurationPropertiesScan;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.util.AntPathMatcher;
import org.springframework.util.PathMatcher;

@Configuration
@ConfigurationPropertiesScan(basePackages = "com.example.gatewayservice.config")
@EnableConfigurationProperties(GatewayAuthProperties.class)
public class GatewayConfig {

    @Bean
    public PathMatcher gatewayPathMatcher() {
        return new AntPathMatcher();
    }
}