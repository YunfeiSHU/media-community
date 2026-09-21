package com.example.gatewayservice.filter;

import java.nio.charset.StandardCharsets;
import java.util.Date;
import java.util.Set;

import org.springframework.cloud.gateway.filter.GatewayFilterChain;
import org.springframework.cloud.gateway.filter.GlobalFilter;
import org.springframework.core.Ordered;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.springframework.stereotype.Component;
import org.springframework.util.AntPathMatcher;
import org.springframework.util.StringUtils;
import org.springframework.web.server.ServerWebExchange;

import com.example.gatewayservice.config.GatewayAuthProperties;
import com.example.gatewayservice.service.RedisBlacklistService;

import io.jsonwebtoken.Claims;
import io.jsonwebtoken.Jws;
import io.jsonwebtoken.JwtException;
import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;
import reactor.core.publisher.Mono;

@Component
public class JwtAuthenticationFilter implements GlobalFilter, Ordered {

    private static final AntPathMatcher PATH_MATCHER = new AntPathMatcher();
    private static final String REQUEST_ID_HEADER = "X-Request-Id";
    private static final String UID_HEADER = "uid";
    private static final String USER_ID_HEADER = "X-User-Id";
    private static final String USER_ROLE_HEADER = "X-User-Role";

    private final GatewayAuthProperties gatewayAuthProperties;
    private final RedisBlacklistService redisBlacklistService;

    public JwtAuthenticationFilter(GatewayAuthProperties gatewayAuthProperties,
                                   RedisBlacklistService redisBlacklistService) {
        this.gatewayAuthProperties = gatewayAuthProperties;
        this.redisBlacklistService = redisBlacklistService;
    }

    @Override
    public Mono<Void> filter(ServerWebExchange exchange, GatewayFilterChain chain) {
        ServerHttpRequest request = exchange.getRequest();
        String path = request.getURI().getPath();

        if (isPermitAll(path)) {
            return chain.filter(exchange);
        }

        String token = extractBearerToken(request.getHeaders().getFirst(HttpHeaders.AUTHORIZATION));
        if (!StringUtils.hasText(token)) {
            return unauthorized(exchange, "Missing JWT token");
        }

        try {
            Claims claims = parseClaims(token);
            if (!gatewayAuthProperties.getJwt().getIssuer().equals(claims.getIssuer())) {
                return unauthorized(exchange, "Invalid JWT issuer");
            }

            Set<String> audience = claims.getAudience();
            if (audience == null || !audience.contains(gatewayAuthProperties.getJwt().getAudience())) {
                return unauthorized(exchange, "Invalid JWT audience");
            }

            Date expiration = claims.getExpiration();
            if (expiration != null && expiration.before(new Date())) {
                return unauthorized(exchange, "Token expired");
            }

            String userId = claims.getSubject();
            if (!StringUtils.hasText(userId)) {
                return unauthorized(exchange, "Missing uid in subject");
            }

            String userRole = claims.get(gatewayAuthProperties.getJwt().getRoleClaim(), String.class);
            if (!StringUtils.hasText(userRole)) {
                userRole = "user";
            }

            final String finalUserRole = userRole;
            String jti = claims.getId();
            if (StringUtils.hasText(jti)) {
                return redisBlacklistService.isBlacklisted(jti)
                        .flatMap(blacklisted -> {
                            if (blacklisted) {
                                return unauthorized(exchange, "Token has been blacklisted");
                            }
                            return proceedWithRequest(exchange, chain, request, userId, finalUserRole);
                        });
            }

            return proceedWithRequest(exchange, chain, request, userId, finalUserRole);
        } catch (JwtException | IllegalArgumentException ex) {
            return unauthorized(exchange, "Invalid JWT token");
        }
    }

    private Mono<Void> proceedWithRequest(ServerWebExchange exchange, GatewayFilterChain chain,
                                          ServerHttpRequest request, String userId, String userRole) {
        final String resolvedUid = userId;
        final String resolvedRole = userRole;

        ServerHttpRequest mutatedRequest = request.mutate()
                .headers(headers -> {
                    headers.remove(REQUEST_ID_HEADER);
                    String requestId = request.getHeaders().getFirst(REQUEST_ID_HEADER);
                    if (StringUtils.hasText(requestId)) {
                        headers.set(REQUEST_ID_HEADER, requestId);
                    }
                    headers.remove(UID_HEADER);
                    headers.remove(USER_ID_HEADER);
                    headers.remove(USER_ROLE_HEADER);
                    headers.set(UID_HEADER, resolvedUid);
                    headers.set(USER_ID_HEADER, resolvedUid);
                    headers.set(USER_ROLE_HEADER, resolvedRole);
                })
                .build();

        return chain.filter(exchange.mutate().request(mutatedRequest).build());
    }

    @Override
    public int getOrder() {
        // 在 IP 限流之后执行
        return Ordered.HIGHEST_PRECEDENCE + 2;
    }

    private boolean isPermitAll(String path) {
        return gatewayAuthProperties.getPermitAll().stream()
                .anyMatch(pattern -> PATH_MATCHER.match(pattern, path));
    }

    private String extractBearerToken(String authorizationHeader) {
        if (!StringUtils.hasText(authorizationHeader) || !authorizationHeader.startsWith("Bearer ")) {
            return null;
        }
        return authorizationHeader.substring(7).trim();
    }

    private Claims parseClaims(String token) {
        byte[] secretBytes = gatewayAuthProperties.getJwt().getSecret().getBytes(StandardCharsets.UTF_8);
        Jws<Claims> jws = Jwts.parser()
                .verifyWith(Keys.hmacShaKeyFor(secretBytes))
                .build()
                .parseSignedClaims(token);
        return jws.getPayload();
    }

    private Mono<Void> unauthorized(ServerWebExchange exchange, String message) {
        exchange.getResponse().setStatusCode(HttpStatus.UNAUTHORIZED);
        exchange.getResponse().getHeaders().set(HttpHeaders.CONTENT_TYPE, "application/json;charset=UTF-8");
        String body = "{\"code\":401,\"message\":\"" + message + "\"}";
        return exchange.getResponse().writeWith(Mono.just(exchange.getResponse().bufferFactory()
                .wrap(body.getBytes(StandardCharsets.UTF_8))));
    }
}
