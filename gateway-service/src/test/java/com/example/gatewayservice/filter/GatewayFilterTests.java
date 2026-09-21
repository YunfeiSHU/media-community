package com.example.gatewayservice.filter;

import java.time.Duration;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.ArgumentMatchers;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import org.springframework.cloud.gateway.filter.GatewayFilterChain;
import org.springframework.data.redis.core.ReactiveStringRedisTemplate;
import org.springframework.data.redis.core.script.RedisScript;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.springframework.mock.http.server.reactive.MockServerHttpRequest;
import org.springframework.mock.web.server.MockServerWebExchange;

import com.example.gatewayservice.config.GatewayAuthProperties;
import com.example.gatewayservice.config.GatewayRateLimitProperties;
import com.example.gatewayservice.service.RedisBlacklistService;

import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;
import reactor.test.StepVerifier;

class GatewayFilterTests {

    private static final String SECRET = "01234567890123456789012345678901";

    private GatewayAuthProperties gatewayAuthProperties;
    private RedisBlacklistService redisBlacklistService;
    private JwtAuthenticationFilter jwtAuthenticationFilter;
    private RequestIdLoggingFilter requestIdLoggingFilter;

    @BeforeEach
    void setUp() {
        gatewayAuthProperties = new GatewayAuthProperties();
        gatewayAuthProperties.getJwt().setSecret(SECRET);
        gatewayAuthProperties.getJwt().setIssuer("auth-service");
        gatewayAuthProperties.getJwt().setAudience("api");
        gatewayAuthProperties.setPermitAll(List.of("/api/v1/auth/**"));

        redisBlacklistService = mock(RedisBlacklistService.class);
        jwtAuthenticationFilter = new JwtAuthenticationFilter(gatewayAuthProperties, redisBlacklistService);
        requestIdLoggingFilter = new RequestIdLoggingFilter();
    }

    @Test
    void jwtFilter_shouldPassPermitAllPathWithoutToken() {
        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/auth/login").build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        when(chain.filter(any())).thenReturn(Mono.empty());

        StepVerifier.create(jwtAuthenticationFilter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(exchange);
    }

    @Test
    void jwtFilter_shouldRejectMissingToken() {
        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me").build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);

        StepVerifier.create(jwtAuthenticationFilter.filter(exchange, chain)).verifyComplete();

        assertThat(exchange.getResponse().getStatusCode()).isEqualTo(HttpStatus.UNAUTHORIZED);
        assertThat(exchange.getResponse().getHeaders().getContentType()).isEqualTo(MediaType.valueOf("application/json;charset=UTF-8"));
    }

    @Test
    void jwtFilter_shouldInjectUserHeadersOnValidToken() {
        String token = buildToken("u-1001", "admin");
        MockServerHttpRequest request = MockServerHttpRequest.get("/api/v1/users/me")
                .header(HttpHeaders.AUTHORIZATION, "Bearer " + token)
                .header("X-Request-Id", "req-123")
                .build();
        MockServerWebExchange exchange = MockServerWebExchange.from(request);
        GatewayFilterChain chain = mock(GatewayFilterChain.class);

        ArgumentCaptor<org.springframework.web.server.ServerWebExchange> captor = ArgumentCaptor.forClass(org.springframework.web.server.ServerWebExchange.class);
        when(chain.filter(any())).thenReturn(Mono.empty());

        StepVerifier.create(jwtAuthenticationFilter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(captor.capture());
        ServerHttpRequest mutatedRequest = captor.getValue().getRequest();
        assertThat(mutatedRequest.getHeaders().getFirst("X-Request-Id")).isEqualTo("req-123");
        assertThat(mutatedRequest.getHeaders().getFirst("uid")).isEqualTo("u-1001");
        assertThat(mutatedRequest.getHeaders().getFirst("X-User-Id")).isEqualTo("u-1001");
        assertThat(mutatedRequest.getHeaders().getFirst("X-User-Role")).isEqualTo("admin");
    }

    @Test
    void jwtFilter_shouldRejectBlacklistedToken() {
        String token = Jwts.builder()
                .id("jti-blacklisted")
                .subject("u-1001")
                .issuer(gatewayAuthProperties.getJwt().getIssuer())
                .audience().add(gatewayAuthProperties.getJwt().getAudience()).and()
                .claim(gatewayAuthProperties.getJwt().getRoleClaim(), "admin")
                .signWith(Keys.hmacShaKeyFor(SECRET.getBytes()))
                .compact();
        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me")
                        .header(HttpHeaders.AUTHORIZATION, "Bearer " + token)
                        .build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        when(redisBlacklistService.isBlacklisted("jti-blacklisted")).thenReturn(Mono.just(true));

        StepVerifier.create(jwtAuthenticationFilter.filter(exchange, chain)).verifyComplete();

        assertThat(exchange.getResponse().getStatusCode()).isEqualTo(HttpStatus.UNAUTHORIZED);
        verify(chain, never()).filter(any());
    }

    @Test
    void jwtFilter_shouldPassWhenJtiNotBlacklisted() {
        String token = Jwts.builder()
                .id("jti-normal")
                .subject("u-1001")
                .issuer(gatewayAuthProperties.getJwt().getIssuer())
                .audience().add(gatewayAuthProperties.getJwt().getAudience()).and()
                .claim(gatewayAuthProperties.getJwt().getRoleClaim(), "user")
                .signWith(Keys.hmacShaKeyFor(SECRET.getBytes()))
                .compact();
        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me")
                        .header(HttpHeaders.AUTHORIZATION, "Bearer " + token)
                        .build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        when(chain.filter(any())).thenReturn(Mono.empty());
        when(redisBlacklistService.isBlacklisted("jti-normal")).thenReturn(Mono.just(false));

        StepVerifier.create(jwtAuthenticationFilter.filter(exchange, chain)).verifyComplete();

        verify(redisBlacklistService).isBlacklisted("jti-normal");
        verify(chain).filter(any());
    }

    @Test
    void requestIdFilter_shouldReuseIncomingRequestId() {
        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me")
                        .header("X-Request-Id", "req-abc")
                        .build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        ArgumentCaptor<org.springframework.web.server.ServerWebExchange> captor = ArgumentCaptor.forClass(org.springframework.web.server.ServerWebExchange.class);
        when(chain.filter(any())).thenReturn(Mono.empty());

        StepVerifier.create(requestIdLoggingFilter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(captor.capture());
        assertThat(captor.getValue().getRequest().getHeaders().getFirst("X-Request-Id")).isEqualTo("req-abc");
    }

    @Test
    void requestIdFilter_shouldGenerateRequestIdWhenMissing() {
        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me").build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        ArgumentCaptor<org.springframework.web.server.ServerWebExchange> captor = ArgumentCaptor.forClass(org.springframework.web.server.ServerWebExchange.class);
        when(chain.filter(any())).thenReturn(Mono.delay(Duration.ofMillis(5)).then());

        StepVerifier.create(requestIdLoggingFilter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(captor.capture());
        String generatedRequestId = captor.getValue().getRequest().getHeaders().getFirst("X-Request-Id");
        assertThat(generatedRequestId).isNotBlank();
    }

    @Test
    void globalRateLimitFilter_shouldAllowRequestWhenTokensAvailable() {
        ReactiveStringRedisTemplate redisTemplate = mock(ReactiveStringRedisTemplate.class);
        GatewayRateLimitProperties properties = new GatewayRateLimitProperties();
        GlobalRateLimitFilter filter = new GlobalRateLimitFilter(redisTemplate, properties);

        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me").build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        when(chain.filter(any())).thenReturn(Mono.empty());

        // 模拟 Redis 脚本返回 1（令牌可用）
        stubRateLimitScript(redisTemplate, Flux.just(1L));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(exchange);
        assertThat(exchange.getResponse().getStatusCode()).isNotEqualTo(HttpStatus.TOO_MANY_REQUESTS);
        // 全局限流使用固定单 key，与请求路径无关
        verify(redisTemplate).execute(
                ArgumentMatchers.<RedisScript<Long>>any(),
                eq(List.of("gateway:rate:global")),
                anyString(), anyString(), anyString());
    }

    @Test
    void globalRateLimitFilter_shouldRejectRequestWhenNoTokens() {
        ReactiveStringRedisTemplate redisTemplate = mock(ReactiveStringRedisTemplate.class);
        GatewayRateLimitProperties properties = new GatewayRateLimitProperties();
        GlobalRateLimitFilter filter = new GlobalRateLimitFilter(redisTemplate, properties);

        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me").build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);

        // 模拟 Redis 脚本返回 0（无令牌）
        stubRateLimitScript(redisTemplate, Flux.just(0L));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        assertThat(exchange.getResponse().getStatusCode()).isEqualTo(HttpStatus.TOO_MANY_REQUESTS);
        verify(chain, never()).filter(any());
    }

    @Test
    void globalRateLimitFilter_shouldPassthroughOnRedisError() {
        ReactiveStringRedisTemplate redisTemplate = mock(ReactiveStringRedisTemplate.class);
        GatewayRateLimitProperties properties = new GatewayRateLimitProperties();
        GlobalRateLimitFilter filter = new GlobalRateLimitFilter(redisTemplate, properties);

        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me").build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        when(chain.filter(any())).thenReturn(Mono.empty());

        // 模拟 Redis 异常
        stubRateLimitScript(redisTemplate, Flux.error(new RuntimeException("Redis connection error")));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        // Redis 异常时应该放行请求
        verify(chain).filter(exchange);
    }

    @Test
    void ipRateLimitFilter_shouldExtractClientIpFromXForwardedFor() {
        ReactiveStringRedisTemplate redisTemplate = mock(ReactiveStringRedisTemplate.class);
        GatewayRateLimitProperties properties = new GatewayRateLimitProperties();
        IpRateLimitFilter filter = new IpRateLimitFilter(redisTemplate, properties);

        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me")
                        .header("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
                        .build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        when(chain.filter(any())).thenReturn(Mono.empty());

        // 模拟 Redis 脚本返回 1（令牌可用）
        stubRateLimitScript(redisTemplate, Flux.just(1L));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(exchange);
        // X-Forwarded-For 含多个 IP 时应取第一个，限流 key 必须落在该 IP 上
        verify(redisTemplate).execute(
                ArgumentMatchers.<RedisScript<Long>>any(),
                eq(List.of("gateway:rate:ip:192.168.1.100")),
                anyString(), anyString(), anyString());
    }

    @Test
    void ipRateLimitFilter_shouldFallbackToRemoteAddressWhenNoXForwardedFor() {
        ReactiveStringRedisTemplate redisTemplate = mock(ReactiveStringRedisTemplate.class);
        GatewayRateLimitProperties properties = new GatewayRateLimitProperties();
        IpRateLimitFilter filter = new IpRateLimitFilter(redisTemplate, properties);

        MockServerWebExchange exchange = MockServerWebExchange.from(
                MockServerHttpRequest.get("/api/v1/users/me")
                        .remoteAddress(new java.net.InetSocketAddress("127.0.0.1", 0))
                        .build());
        GatewayFilterChain chain = mock(GatewayFilterChain.class);
        when(chain.filter(any())).thenReturn(Mono.empty());

        // 模拟 Redis 脚本返回 1（令牌可用）
        stubRateLimitScript(redisTemplate, Flux.just(1L));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(exchange);
        // 无代理头时应回退到 TCP 连接对端地址
        verify(redisTemplate).execute(
                ArgumentMatchers.<RedisScript<Long>>any(),
                eq(List.of("gateway:rate:ip:127.0.0.1")),
                anyString(), anyString(), anyString());
    }

    @Test
    void globalRateLimitFilter_shouldHaveHighestPrecedence() {
        ReactiveStringRedisTemplate redisTemplate = mock(ReactiveStringRedisTemplate.class);
        GatewayRateLimitProperties properties = new GatewayRateLimitProperties();
        GlobalRateLimitFilter filter = new GlobalRateLimitFilter(redisTemplate, properties);

        assertThat(filter.getOrder()).isEqualTo(Integer.MIN_VALUE);
    }

    @Test
    void ipRateLimitFilter_shouldHavePrecedenceAfterGlobalLimit() {
        ReactiveStringRedisTemplate redisTemplate = mock(ReactiveStringRedisTemplate.class);
        GatewayRateLimitProperties properties = new GatewayRateLimitProperties();
        IpRateLimitFilter filter = new IpRateLimitFilter(redisTemplate, properties);

        assertThat(filter.getOrder()).isEqualTo(Integer.MIN_VALUE + 1);
    }

    private String buildToken(String subject, String role) {
        return Jwts.builder()
                .subject(subject)
                .issuer(gatewayAuthProperties.getJwt().getIssuer())
                .audience().add(gatewayAuthProperties.getJwt().getAudience()).and()
                .claim(gatewayAuthProperties.getJwt().getRoleClaim(), role)
                .signWith(Keys.hmacShaKeyFor(SECRET.getBytes()))
                .compact();
    }

    /**
     * 统一 stub 限流 Lua 脚本的返回值。
     *
     * <p>使用 {@code ArgumentMatchers.<RedisScript<Long>>any()} 显式给出类型参数，
     * 避免裸类型 {@code RedisScript.class} 使 {@code execute(...)} 与 {@code when(...)}
     * 的类型推断退化为 raw type（raw {@code RedisScript}、raw {@code Flux}、raw
     * {@code OngoingStubbing}），从而产生一连串 unchecked 告警。</p>
     *
     * @param redisTemplate 被测过滤器持有的 Redis 模板 mock
     * @param scriptResult  脚本返回的令牌判定结果，1 表示放行、0 表示限流，异常表示 Redis 故障
     */
    private void stubRateLimitScript(ReactiveStringRedisTemplate redisTemplate, Flux<Long> scriptResult) {
        when(redisTemplate.execute(
                ArgumentMatchers.<RedisScript<Long>>any(),
                anyList(),
                anyString(), anyString(), anyString()))
                .thenReturn(scriptResult);
    }
}