package com.example.gatewayservice.config;

import org.springframework.boot.context.properties.ConfigurationPropertiesScan;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.util.AntPathMatcher;
import org.springframework.util.PathMatcher;
import org.springframework.web.client.RestTemplate;
import org.springframework.web.reactive.function.client.WebClient;

@Configuration
@ConfigurationPropertiesScan(basePackages = "com.example.gatewayservice.config")
@EnableConfigurationProperties({GatewayAuthProperties.class, GatewayRateLimitProperties.class})
public class GatewayConfig {

    @Bean
    public PathMatcher gatewayPathMatcher() {
        return new AntPathMatcher();
    }

    @Bean
    public RestTemplate restTemplate() {
        return new RestTemplate();
    }

    @Bean
    public WebClient.Builder webClientBuilder() {
        return WebClient.builder();
    }
}