package com.example.gatewayservice.filter;

import org.springframework.cloud.gateway.filter.GatewayFilter;
import org.springframework.cloud.gateway.filter.factory.AbstractGatewayFilterFactory;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.springframework.stereotype.Component;

@Component
public class UserInfoInjectionFilter extends AbstractGatewayFilterFactory<UserInfoInjectionFilter.Config> {

    public UserInfoInjectionFilter() {
        super(Config.class);
    }

    @Override
    public GatewayFilter apply(Config config) {
        return (exchange, chain) -> {
            ServerHttpRequest request = exchange.getRequest();
            
            // 从请求头获取用户信息（JWT认证过滤器已经注入）
            String uid = request.getHeaders().getFirst("X-User-Id");
            String userRole = request.getHeaders().getFirst("X-User-Role");
            
            // 如果没有用户信息，说明未认证，放行给后续处理
            if (uid == null) {
                return chain.filter(exchange);
            }
            
            // 构建新请求，确保下游服务能获取到用户信息
            ServerHttpRequest newRequest = request.mutate()
                    .header("X-User-Id", uid)
                    .header("X-User-Role", userRole != null ? userRole : "user")
                    .build();
            
            return chain.filter(exchange.mutate().request(newRequest).build());
        };
    }

    public static class Config {
        // 配置类，可根据需要添加属性
    }
}
