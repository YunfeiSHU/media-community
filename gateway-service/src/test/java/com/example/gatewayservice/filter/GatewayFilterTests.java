package com.example.gatewayservice.filter;

import java.time.Duration;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyList;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.mock;
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
        when(redisTemplate.execute(any(RedisScript.class), anyList(), anyString(), anyString(), anyString()))
                .thenReturn(Flux.just(1L));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(exchange);
        assertThat(exchange.getResponse().getStatusCode()).isNotEqualTo(HttpStatus.TOO_MANY_REQUESTS);
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
        when(redisTemplate.execute(any(RedisScript.class), anyList(), anyString(), anyString(), anyString()))
                .thenReturn(Flux.just(0L));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        assertThat(exchange.getResponse().getStatusCode()).isEqualTo(HttpStatus.TOO_MANY_REQUESTS);
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
        when(redisTemplate.execute(any(RedisScript.class), anyList(), anyString(), anyString(), anyString()))
                .thenReturn(Flux.error(new RuntimeException("Redis connection error")));

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
        when(redisTemplate.execute(any(RedisScript.class), anyList(), anyString(), anyString(), anyString()))
                .thenReturn(Flux.just(1L));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(exchange);
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
        when(redisTemplate.execute(any(RedisScript.class), anyList(), anyString(), anyString(), anyString()))
                .thenReturn(Flux.just(1L));

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        verify(chain).filter(exchange);
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
}