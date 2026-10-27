package com.example.gatewayservice.filter;

import java.net.URI;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.ValueSource;
import static org.mockito.Mockito.mock;
import org.springframework.boot.context.properties.bind.Bindable;
import org.springframework.boot.context.properties.bind.Binder;
import org.springframework.boot.context.properties.source.ConfigurationPropertySources;
import org.springframework.boot.env.YamlPropertySourceLoader;
import org.springframework.cloud.gateway.filter.GatewayFilterChain;
import org.springframework.core.env.MutablePropertySources;
import org.springframework.core.env.PropertySource;
import org.springframework.core.io.ClassPathResource;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpMethod;
import org.springframework.http.HttpStatus;
import org.springframework.mock.http.server.reactive.MockServerHttpRequest;
import org.springframework.mock.web.server.MockServerWebExchange;
import org.springframework.web.server.ServerWebExchange;

import com.example.gatewayservice.config.GatewayAuthProperties;
import com.example.gatewayservice.service.RedisBlacklistService;

import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.security.Keys;
import reactor.core.publisher.Mono;
import reactor.test.StepVerifier;

/**
 * 网关对 <b>Go 体系</b>（video / live / danmaku）的鉴权分类测试。
 *
 * <p>与其他网关测试不同，这里的白名单与「查询参数令牌」配置<b>直接读取真实的
 * {@code src/main/resources/application.yml}</b>，而不是在测试里另写一份。
 * 目的是让「配置与真实接口路径不一致」这类问题在构建阶段就暴露 ——
 * 历史上就出现过白名单写成 {@code /api/v1/auth/register} 而真实路径是
 * {@code /api/v1/auth/register/phone}，导致经网关注册一律 401。
 */
class GatewayGoRoutingTests {

    private static final String SECRET = "01234567890123456789012345678901";

    private GatewayAuthProperties gatewayAuthProperties;
    private JwtAuthenticationFilter filter;

    @BeforeEach
    void setUp() {
        gatewayAuthProperties = new GatewayAuthProperties();
        gatewayAuthProperties.getJwt().setSecret(SECRET);
        gatewayAuthProperties.getJwt().setIssuer("auth-service");
        gatewayAuthProperties.getJwt().setAudience("api");

        // 从真实的 application.yml 读取网关鉴权配置，避免测试与生产配置漂移
        gatewayAuthProperties.setPermitAll(loadList("gateway.auth.permit-all"));
        gatewayAuthProperties.setOptionalAuth(loadList("gateway.auth.optional-auth"));
        gatewayAuthProperties.getJwt().setTokenQueryPaths(loadList("gateway.auth.jwt.token-query-paths"));

        filter = new JwtAuthenticationFilter(gatewayAuthProperties, mock(RedisBlacklistService.class));
    }

    // ------------------------------------------------------------------ 配置读取

    private static Binder binder() {
        try {
            List<PropertySource<?>> sources = new YamlPropertySourceLoader()
                    .load("application", new ClassPathResource("application.yml"));
            MutablePropertySources propertySources = new MutablePropertySources();
            sources.forEach(propertySources::addLast);
            return new Binder(ConfigurationPropertySources.from(propertySources));
        }
        catch (Exception ex) {
            throw new IllegalStateException("读取 application.yml 失败", ex);
        }
    }

    private static List<String> loadList(String key) {
        return binder().bind(key, Bindable.listOf(String.class)).orElse(List.of());
    }

    private static String loadString(String key) {
        return binder().bind(key, Bindable.of(String.class)).orElse("");
    }

    // ------------------------------------------------------------------ 放行：无需令牌

    @DisplayName("Go 体系公开接口与修正后的 auth 路径：不带令牌即可通过")
    @ParameterizedTest(name = "{0} {1}")
    @CsvSource({
            // --- auth-service：注册链路（历史上 3 条路径写错，导致注册全 401）---
            "POST, /api/v1/auth/login",
            "POST, /api/v1/auth/token/refresh",
            "POST, /api/v1/auth/verification/send",
            "POST, /api/v1/auth/register/phone",
            "POST, /api/v1/auth/register/email",
            // --- video-service-go：分类与视频读接口 ---
            "GET,  /api/v1/categories",
            "GET,  /api/v1/videos",
            "GET,  /api/v1/videos/1001",
            "GET,  /api/v1/videos/1001/play",
            // --- live-service-go：直播间读接口 + 心跳 + SRS 回调 + 联调接口 ---
            "GET,  /api/v1/live/rooms",
            "GET,  /api/v1/live/rooms/5001",
            "GET,  /api/v1/live/rooms/5001/online",
            "GET,  /api/v1/live/rooms/5001/play",
            "POST, /api/v1/live/rooms/5001/heartbeat",
            "POST, /api/v1/live/hooks/on-publish",
            "POST, /api/v1/live/hooks/on-unpublish",
            "POST, /api/v1/live/hooks/on-play",
            "POST, /api/v1/live/hooks/on-stop",
            "POST, /api/v1/live/dev/publish/stream-key-1",
            "POST, /api/v1/live/dev/unpublish/stream-key-1",
            // --- danmaku-service-go：历史与统计 ---
            "GET,  /api/v1/danmaku/rooms/5001/history",
            "GET,  /api/v1/danmaku/videos/1001/history",
            "GET,  /api/v1/danmaku/stats",
            // --- 既有 Java 侧行为不应回归 ---
            "GET,  /api/v1/products/P1001",
            "GET,  /api/v1/skus/S1001",
            "POST, /api/v1/payments/callback"
    })
    void shouldPassWithoutToken(String method, String path) {
        ServerWebExchange captured = forward(method, path.trim(), null);

        assertThat(captured.getRequest().getHeaders().getFirst("X-User-Id")).isNull();
    }

    // ------------------------------------------------------------------ 拦截：必须携带令牌

    @DisplayName("Go 体系写接口 / 管理接口 / WebSocket：不带令牌一律 401")
    @ParameterizedTest(name = "{0} {1}")
    @CsvSource({
            // video：写接口与转码
            "POST,   /api/v1/categories",
            "POST,   /api/v1/videos",
            "PUT,    /api/v1/videos/1001",
            "DELETE, /api/v1/videos/1001",
            "POST,   /api/v1/videos/1001/upload",
            "POST,   /api/v1/videos/1001/publish",
            "POST,   /api/v1/videos/1001/offline",
            "GET,    /api/v1/videos/1001/transcode",
            "POST,   /api/v1/videos/1001/transcode/retry",
            // live：写接口与推流密钥
            "POST,   /api/v1/live/rooms",
            "PUT,    /api/v1/live/rooms/5001",
            "DELETE, /api/v1/live/rooms/5001",
            "GET,    /api/v1/live/rooms/5001/stream-key",
            "GET,    /api/v1/live/rooms/5001/records",
            "POST,   /api/v1/live/rooms/5001/start",
            "POST,   /api/v1/live/rooms/5001/stop",
            "POST,   /api/v1/live/rooms/5001/force-close",
            // danmaku：发送 / 撤回 / 管理
            "POST,   /api/v1/danmaku/send",
            "POST,   /api/v1/danmaku/rooms/5001/send",
            "DELETE, /api/v1/danmaku/dm-1",
            "POST,   /api/v1/danmaku/admin/reload-words",
            "GET,    /api/v1/danmaku/admin/words",
            "POST,   /api/v1/danmaku/admin/words"
    })
    void shouldRequireToken(String method, String path) {
        MockServerWebExchange exchange = exchange(method, path.trim(), null);
        ChainSpy chain = new ChainSpy();

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        assertThat(exchange.getResponse().getStatusCode())
                .as("%s %s 必须鉴权", method, path.trim())
                .isEqualTo(HttpStatus.UNAUTHORIZED);
        assertThat(chain.invoked).as("不应把请求转发给下游").isFalse();
    }

    // ------------------------------------------------------------------ WebSocket 查询参数令牌

    @Test
    @DisplayName("WebSocket：允许用查询参数 access_token 完成鉴权，并注入可信身份头")
    void wsShouldAcceptTokenFromQueryParam() {
        String token = buildToken("u-2001", "user");
        ServerWebExchange captured = forward("GET", "/ws/danmaku?roomId=5001&access_token=" + token, null);

        assertThat(captured.getRequest().getHeaders().getFirst("X-User-Id")).isEqualTo("u-2001");
        assertThat(captured.getRequest().getHeaders().getFirst("X-User-Role")).isEqualTo("user");
    }

    @Test
    @DisplayName("WebSocket：转发前必须把 access_token 从 URL 上摘掉，其余参数连编码态一起保留")
    void wsShouldStripTokenFromForwardedQuery() {
        String token = buildToken("u-2001", "user");
        ServerWebExchange captured = forward("GET",
                "/ws/danmaku?roomId=5001&nickname=%E5%B0%8F%E6%98%8E&access_token=" + token, null);

        // 必须看 rawQuery：URI.getQuery() 会把 %E5%B0%8F%E6%98%8E 解码成中文，
        // 用它会误判成「中文被原样透传」，从而掩盖真正的编码问题
        String rawQuery = captured.getRequest().getURI().getRawQuery();
        assertThat(rawQuery).doesNotContain("access_token").doesNotContain(token);
        // roomId 是握手必需参数，nickname 必须保持百分号编码（否则下游收到的 URI 非法）
        assertThat(rawQuery).contains("roomId=5001").contains("nickname=%E5%B0%8F%E6%98%8E");
    }

    @Test
    @DisplayName("WebSocket：?uid= 伪造无效，身份仍以 JWT 的 sub 为准")
    void wsShouldIgnoreForgedUidInQueryParams() {
        String token = buildToken("u-2001", "user");
        ServerWebExchange captured = forward("GET",
                "/ws/danmaku?roomId=5001&uid=forged-uid&access_token=" + token, null);

        // Go 侧是「请求头优先于 query」，因此伪造的 ?uid= 无法生效
        assertThat(captured.getRequest().getHeaders().getFirst("X-User-Id")).isEqualTo("u-2001");
    }

    @DisplayName("仅 token-query-paths 命中的路径可从查询参数取令牌，其他路径一概不行")
    @ParameterizedTest(name = "POST {0}")
    @ValueSource(strings = {
            "/api/v1/videos",
            "/api/v1/live/rooms/5001/start",
            "/api/v1/danmaku/send",
            "/api/v1/users/me"
    })
    void shouldNotAcceptQueryTokenOnNonWebSocketPaths(String path) {
        String token = buildToken("u-2001", "user");
        // URL 上带了合法令牌，但该路径不允许从查询参数取令牌
        MockServerWebExchange exchange = exchange("POST", path + "?access_token=" + token, null);
        ChainSpy chain = new ChainSpy();

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        assertThat(exchange.getResponse().getStatusCode())
                .as("%s 不应接受查询参数令牌", path)
                .isEqualTo(HttpStatus.UNAUTHORIZED);
        assertThat(chain.invoked).isFalse();
    }

    // ------------------------------------------------------------------ 可选登录

    @DisplayName("可选登录：不带令牌放行，且不注入任何身份头")
    @ParameterizedTest(name = "{0} {1}")
    @CsvSource({
            "GET,  /api/v1/videos",
            "GET,  /api/v1/videos/1001",
            "GET,  /api/v1/videos/1001/play",
            "GET,  /api/v1/videos/1001/interactions",
            "GET,  /api/v1/videos/interactions?ids=1,2",
            "GET,  /api/v1/live/rooms",
            "GET,  /api/v1/live/rooms/5001",
            "GET,  /api/v1/users/1001/followers",
            "GET,  /api/v1/users/1001/following"
    })
    void optionalAuthPathsShouldPassAsGuest(String method, String path) {
        ServerWebExchange captured = forward(method, path.trim(), null);

        assertThat(captured.getRequest().getHeaders().getFirst("X-User-Id")).isNull();
        assertThat(captured.getRequest().getHeaders().getFirst("X-User-Role")).isNull();
    }

    @Test
    @DisplayName("可选登录：带合法令牌时仍要校验并注入身份（?mine=true / ?favorite=true 依赖它）")
    void optionalAuthPathsShouldInjectIdentityWhenTokenPresent() {
        String token = buildToken("u-1001", "user");

        ServerWebExchange captured = forward("GET", "/api/v1/videos?mine=true", token);

        assertThat(captured.getRequest().getHeaders().getFirst("X-User-Id")).isEqualTo("u-1001");
        assertThat(captured.getRequest().getHeaders().getFirst("X-User-Role")).isEqualTo("user");
    }

    @Test
    @DisplayName("可选登录：令牌非法时返回 401，不能静默降级成游客")
    void optionalAuthPathsShouldRejectInvalidToken() {
        MockServerWebExchange exchange = exchange("GET", "/api/v1/videos?favorite=true", "not-a-jwt");
        ChainSpy chain = new ChainSpy();

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        assertThat(exchange.getResponse().getStatusCode()).isEqualTo(HttpStatus.UNAUTHORIZED);
        assertThat(chain.invoked).isFalse();
    }

    @Test
    @DisplayName("配置自检：视频/直播读接口必须在 optional-auth，不能退回 permit-all（否则 mine=true 恒 401）")
    void videoReadPathsMustBeOptionalAuthNotPermitAll() {
        assertThat(gatewayAuthProperties.getPermitAll())
                .doesNotContain("GET /api/v1/videos", "GET /api/v1/videos/*",
                        "GET /api/v1/live/rooms", "GET /api/v1/live/rooms/*");
        assertThat(gatewayAuthProperties.getOptionalAuth())
                .contains("GET /api/v1/videos", "GET /api/v1/videos/*");
    }

    @Test
    @DisplayName("配置自检：上传路由必须覆盖 5 分钟超时，否则大分片必然 504")
    void uploadRouteMustOverrideResponseTimeout() {
        // 全局 response-timeout 是 10s；分片上传单请求可能 40s+
        assertThat(loadString("spring.cloud.gateway.routes[4].metadata.response-timeout"))
                .isEqualTo("300000");
    }

    // ------------------------------------------------------------------ 配置自检

    @Test
    @DisplayName("WebSocket：游客不带令牌也能连（只读），带令牌则注入身份")
    void wsShouldAllowGuestButStillInjectIdentity() {
        ServerWebExchange guest = forward("GET", "/ws/danmaku?roomId=5001", null);
        assertThat(guest.getRequest().getHeaders().getFirst("X-User-Id")).isNull();

        String token = buildToken("u-2001", "user");
        ServerWebExchange authed = forward("GET", "/ws/danmaku?roomId=5001&access_token=" + token, null);
        assertThat(authed.getRequest().getHeaders().getFirst("X-User-Id")).isEqualTo("u-2001");
    }

    @Test
    @DisplayName("配置自检：WebSocket 路径必须走查询参数令牌，且必须不在白名单里")
    void webSocketPathMustBeAuthenticatedViaQueryToken() {
        // 若 /ws/danmaku 进了白名单，JWT 会被整体跳过 —— 那就等于只靠 ?uid= 自证身份，可被任意伪造
        assertThat(gatewayAuthProperties.getPermitAll()).doesNotContain("/ws/danmaku");
        assertThat(gatewayAuthProperties.getJwt().getTokenQueryPaths()).contains("/ws/danmaku");
    }

    @Test
    @DisplayName("配置自检：auth 白名单必须与 AuthController 的真实路径一致，且不留历史错误路径")
    void authPermitAllPathsMustMatchRealControllerMappings() {
        List<String> permitAll = gatewayAuthProperties.getPermitAll();

        // AuthController 里真实存在的 @PostMapping
        assertThat(permitAll).contains(
                "POST /api/v1/auth/login",
                "POST /api/v1/auth/token/refresh",
                "POST /api/v1/auth/verification/send",
                "POST /api/v1/auth/register/phone",
                "POST /api/v1/auth/register/email");

        // 历史上写错的路径：仓库里根本不存在这些映射，留着只会造成「以为放行了其实没有」
        assertThat(permitAll).doesNotContain("/api/v1/auth/refresh", "/api/v1/auth/register");
    }

    @Test
    @DisplayName("配置自检：Go 三服务路由必须存在且为直连地址（它们不在 Nacos 里，lb:// 必然失败）")
    void goServiceRoutesMustBeDeclaredWithDirectUri() {
        assertThat(loadString("spring.cloud.gateway.routes[4].id")).isEqualTo("video-service-go");
        assertThat(loadString("spring.cloud.gateway.routes[5].id")).isEqualTo("live-service-go");
        assertThat(loadString("spring.cloud.gateway.routes[6].id")).isEqualTo("danmaku-service-go");

        assertThat(loadString("spring.cloud.gateway.routes[4].uri"))
                .contains("localhost:9001").doesNotContain("lb://");
        assertThat(loadString("spring.cloud.gateway.routes[5].uri"))
                .contains("localhost:9002").doesNotContain("lb://");
        assertThat(loadString("spring.cloud.gateway.routes[6].uri"))
                .contains("localhost:9003").doesNotContain("lb://");

        // WebSocket 入口必须在路由断言里，否则长连接根本到不了下游
        assertThat(loadString("spring.cloud.gateway.routes[6].predicates[0]")).contains("/ws/danmaku");
        // 视频服务的断言必须同时覆盖分类与视频两条前缀
        assertThat(loadString("spring.cloud.gateway.routes[4].predicates[0]"))
                .contains("/api/v1/categories/**").contains("/api/v1/videos/**");
    }

    // ------------------------------------------------------------------ 辅助

    /** 跑一次过滤链；返回被转发出去的 exchange，若被拦截则直接让断言失败。 */
    private ServerWebExchange forward(String method, String path, String bearerToken) {
        MockServerWebExchange exchange = exchange(method, path, bearerToken);
        ChainSpy chain = new ChainSpy();

        StepVerifier.create(filter.filter(exchange, chain)).verifyComplete();

        assertThat(exchange.getResponse().getStatusCode())
                .as("%s %s 不应被拦截", method, path)
                .isNotEqualTo(HttpStatus.UNAUTHORIZED);
        assertThat(chain.forwarded).as("请求应被转发").isNotNull();
        return chain.forwarded;
    }

    private static MockServerWebExchange exchange(String method, String path, String bearerToken) {
        MockServerHttpRequest.BaseBuilder<?> builder = MockServerHttpRequest
                .method(HttpMethod.valueOf(method.trim()), URI.create(path));
        if (bearerToken != null) {
            builder.header(HttpHeaders.AUTHORIZATION, "Bearer " + bearerToken);
        }
        return MockServerWebExchange.from(builder.build());
    }

    private String buildToken(String uid, String role) {
        return Jwts.builder()
                .subject(uid)
                .issuer(gatewayAuthProperties.getJwt().getIssuer())
                .audience().add(gatewayAuthProperties.getJwt().getAudience()).and()
                .claim(gatewayAuthProperties.getJwt().getRoleClaim(), role)
                .signWith(Keys.hmacShaKeyFor(SECRET.getBytes()))
                .compact();
    }

    /** 既能当 {@link GatewayFilterChain} 用，又能记录是否被调用、以及转发出去的 exchange。 */
    private static final class ChainSpy implements GatewayFilterChain {

        private boolean invoked;

        private ServerWebExchange forwarded;

        @Override
        public Mono<Void> filter(ServerWebExchange exchange) {
            this.invoked = true;
            this.forwarded = exchange;
            return Mono.empty();
        }
    }
}
