package com.example.gatewayservice.filter;

import java.nio.charset.StandardCharsets;
import java.net.URI;
import java.util.Date;
import java.util.List;
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
import org.springframework.web.util.UriComponentsBuilder;

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

    /** 网关注入的用户标识头，唯一来源是 JWT 标准 claim {@code sub} */
    private static final String USER_ID_HEADER = "X-User-Id";

    /** 网关注入的角色头 */
    private static final String USER_ROLE_HEADER = "X-User-Role";

    /** 历史非标准头，网关不再注入，仅用于清除客户端伪造值 */
    private static final String LEGACY_UID_HEADER = "uid";

    /**
     * 查询参数令牌名。仅对 {@code gateway.auth.jwt.token-query-paths} 命中的路径生效，
     * 用于浏览器 WebSocket 握手（浏览器无法为 {@code new WebSocket(url)} 设置请求头）。
     */
    private static final String TOKEN_QUERY_PARAM = "access_token";

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
        String method = request.getMethod() != null ? request.getMethod().name() : "";

        boolean forceAuth = matchesAny(gatewayAuthProperties.getForceAuth(), path, method);
        if (!forceAuth && isPermitAll(path, method)) {
            // 白名单放行前同样清洗身份头，避免客户端伪造 uid / X-User-Id / X-User-Role 直通下游
            return chain.filter(stripIdentityHeaders(exchange));
        }

        // 可选登录：没有令牌也放行，但下游只能看到“游客”；令牌非法时仍按 401 处理。
        boolean optionalAuthPath = matchesAny(gatewayAuthProperties.getOptionalAuth(), path, method);

        String token = extractBearerToken(request.getHeaders().getFirst(HttpHeaders.AUTHORIZATION));
        boolean tokenFromQuery = false;
        if (!StringUtils.hasText(token) && isTokenQueryPath(path)) {
            token = request.getQueryParams().getFirst(TOKEN_QUERY_PARAM);
            tokenFromQuery = StringUtils.hasText(token);
        }
        if (!StringUtils.hasText(token)) {
            if (optionalAuthPath) {
                return chain.filter(stripIdentityHeaders(exchange));
            }
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

            // uid 固定取 JWT 标准 claim `sub`：auth-service 签发时把用户 uid 写进 sub，
            // 不使用数据库自增主键，也没有可配置的 claim 名（见 gateway.auth.jwt 注释）
            String userId = claims.getSubject();
            if (!StringUtils.hasText(userId)) {
                return unauthorized(exchange, "Missing uid in subject");
            }

            String userRole = claims.get(gatewayAuthProperties.getJwt().getRoleClaim(), String.class);
            if (!StringUtils.hasText(userRole)) {
                userRole = "user";
            }

            final String finalUserRole = userRole;
            final boolean stripTokenFromQuery = tokenFromQuery;
            String jti = claims.getId();
            if (StringUtils.hasText(jti)) {
                return redisBlacklistService.isBlacklisted(jti)
                        .flatMap(blacklisted -> {
                            if (blacklisted) {
                                return unauthorized(exchange, "Token has been blacklisted");
                            }
                            return proceedWithRequest(exchange, chain, request, userId, finalUserRole,
                                    stripTokenFromQuery);
                        });
            }

            return proceedWithRequest(exchange, chain, request, userId, finalUserRole, stripTokenFromQuery);
        } catch (JwtException | IllegalArgumentException ex) {
            return unauthorized(exchange, "Invalid JWT token");
        }
    }

    private Mono<Void> proceedWithRequest(ServerWebExchange exchange, GatewayFilterChain chain,
                                          ServerHttpRequest request, String userId, String userRole,
                                          boolean stripTokenFromQuery) {
        // 令牌来自 URL 时必须在下游之前摘掉，否则会随访问日志 / 反代日志泄漏。
        // build(true) 表示查询串已是编码态，避免对 roomId、nickname 里的中文重复编码。
        URI sanitizedUri = stripTokenFromQuery
                ? UriComponentsBuilder.fromUri(request.getURI())
                        .replaceQueryParam(TOKEN_QUERY_PARAM)
                        .build(true)
                        .toUri()
                : null;

        ServerHttpRequest mutatedRequest = request.mutate()
                .uri(sanitizedUri != null ? sanitizedUri : request.getURI())
                .headers(headers -> {
                    headers.remove(REQUEST_ID_HEADER);
                    String requestId = request.getHeaders().getFirst(REQUEST_ID_HEADER);
                    if (StringUtils.hasText(requestId)) {
                        headers.set(REQUEST_ID_HEADER, requestId);
                    }
                    // 先清掉客户端可能伪造的身份头，再写入网关校验后的可信值
                    removeIdentityHeaders(headers);
                    headers.set(USER_ID_HEADER, userId);
                    headers.set(USER_ROLE_HEADER, userRole);
                })
                .build();

        return chain.filter(exchange.mutate().request(mutatedRequest).build());
    }

    @Override
    public int getOrder() {
        // 在 IP 限流之后执行
        return Ordered.HIGHEST_PRECEDENCE + 2;
    }

    /**
     * 判断请求是否命中免鉴权白名单。
     *
     * <p>规则支持 {@code METHOD /path} 前缀写法，例如 {@code GET /api/v1/products/*}
     * 表示只有 GET 方法免鉴权；不带方法前缀时对任意方法放行。
     */
    private boolean isPermitAll(String path, String method) {
        return matchesAny(gatewayAuthProperties.getPermitAll(), path, method);
    }

    /**
     * 判断请求是否命中「可选登录」列表。
     *
     * <p>命中只改变“无令牌时是否放行”，令牌存在时仍走完整校验，不能用来绕过鉴权。
     */
    private boolean matchesAny(List<String> rules, String path, String method) {
        for (String rule : rules) {
            if (!StringUtils.hasText(rule)) {
                continue;
            }
            String pattern = rule.trim();
            int separator = pattern.indexOf(' ');
            if (separator > 0) {
                String requiredMethod = pattern.substring(0, separator).trim();
                pattern = pattern.substring(separator + 1).trim();
                if (!requiredMethod.equalsIgnoreCase(method)) {
                    continue;
                }
            }
            if (PATH_MATCHER.match(pattern, path)) {
                return true;
            }
        }
        return false;
    }

    /**
     * 判断该路径是否允许从查询参数取令牌（浏览器 WebSocket 握手专用）。
     *
     * <p>命中不代表免鉴权：令牌来源只是从 {@code Authorization} 头换成查询参数，
     * 后续的签名 / 签发方 / 受众 / 过期 / 黑名单校验一个都不会跳过。
     */
    private boolean isTokenQueryPath(String path) {
        for (String pattern : gatewayAuthProperties.getJwt().getTokenQueryPaths()) {
            if (StringUtils.hasText(pattern) && PATH_MATCHER.match(pattern.trim(), path)) {
                return true;
            }
        }
        return false;
    }

    /**
     * 移除客户端可能伪造的身份头。
     *
     * <p>仅用于白名单分支：受保护路径由 {@link #proceedWithRequest} 覆盖写入可信身份头。
     */
    private ServerWebExchange stripIdentityHeaders(ServerWebExchange exchange) {
        ServerHttpRequest sanitized = exchange.getRequest().mutate()
                .headers(this::removeIdentityHeaders)
                .build();
        return exchange.mutate().request(sanitized).build();
    }

    /**
     * 清除身份头（含历史非标准头 {@code uid}），确保下游只能看到网关注入的可信身份。
     */
    private void removeIdentityHeaders(HttpHeaders headers) {
        headers.remove(USER_ID_HEADER);
        headers.remove(USER_ROLE_HEADER);
        headers.remove(LEGACY_UID_HEADER);
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

    /**
     * 未认证的错误码：与 auth / user / product / order 四个下游服务的
     * `ErrorCode.UNAUTHORIZED(40101)` 保持一致，前端可用同一套错误码处理。
     */
    private static final int UNAUTHORIZED_CODE = 40101;

    private Mono<Void> unauthorized(ServerWebExchange exchange, String message) {
        exchange.getResponse().setStatusCode(HttpStatus.UNAUTHORIZED);
        exchange.getResponse().getHeaders().set(HttpHeaders.CONTENT_TYPE, "application/json;charset=UTF-8");
        String body = "{\"code\":" + UNAUTHORIZED_CODE + ",\"message\":\"" + message + "\",\"data\":null}";
        return exchange.getResponse().writeWith(Mono.just(exchange.getResponse().bufferFactory()
                .wrap(body.getBytes(StandardCharsets.UTF_8))));
    }
}
