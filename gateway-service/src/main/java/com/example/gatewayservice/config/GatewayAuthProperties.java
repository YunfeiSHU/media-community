package com.example.gatewayservice.config;

import java.util.ArrayList;
import java.util.List;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * 网关认证相关配置，对应 application.yml 中的 gateway.auth.*
 */
@ConfigurationProperties(prefix = "gateway.auth")
public class GatewayAuthProperties {

    /**
     * 无需认证即可访问的路径，Ant 风格。支持两种写法：
     * <ul>
     *     <li>{@code /path/**} —— 任意 HTTP 方法均可访问；</li>
     *     <li>{@code GET /path/*} —— 仅指定 HTTP 方法可访问，其余方法仍执行 JWT 校验。</li>
     * </ul>
     */
    private List<String> permitAll = new ArrayList<>();

    private Jwt jwt = new Jwt();

    public List<String> getPermitAll() {
        return permitAll;
    }

    public void setPermitAll(List<String> permitAll) {
        this.permitAll = permitAll;
    }

    public Jwt getJwt() {
        return jwt;
    }

    public void setJwt(Jwt jwt) {
        this.jwt = jwt;
    }

    /**
     * JWT 校验配置。
     *
     * <p>用户 ID <b>固定</b>取 JWT 标准 claim {@code sub}（auth-service 签发时把用户 uid 写进 sub），
     * 因此这里不再提供「uid 所在 claim 名」的配置项，避免配置与实现不一致；角色取 {@link #roleClaim}。
     */
    public static class Jwt {
        /** 签名密钥，HS256 要求至少 32 字节 */
        private String secret;

        /** 允许的签发方 */
        private String issuer = "auth-service";

        /** 允许的受众 */
        private String audience = "api";

        /** 角色所在的 claim 名 */
        private String roleClaim = "role";

        /**
         * 允许从查询参数（{@code access_token}）取令牌的路径，Ant 风格。
         *
         * <p>只用于浏览器 WebSocket：浏览器 {@code new WebSocket(url)} 无法自定义请求头，
         * 只能把令牌放在 URL 上。命中这些路径时令牌来源改为查询参数，
         * 但**仍需通过全部 JWT 校验**；转发前会把该参数从 URL 摘掉，避免泄漏进下游日志。
         * 未列出的路径只能走 {@code Authorization} 头。
         */
        private List<String> tokenQueryPaths = new ArrayList<>();

        public String getSecret() {
            return secret;
        }

        public void setSecret(String secret) {
            this.secret = secret;
        }

        public String getIssuer() {
            return issuer;
        }

        public void setIssuer(String issuer) {
            this.issuer = issuer;
        }

        public String getAudience() {
            return audience;
        }

        public void setAudience(String audience) {
            this.audience = audience;
        }

        public String getRoleClaim() {
            return roleClaim;
        }

        public void setRoleClaim(String roleClaim) {
            this.roleClaim = roleClaim;
        }

        public List<String> getTokenQueryPaths() {
            return tokenQueryPaths;
        }

        public void setTokenQueryPaths(List<String> tokenQueryPaths) {
            this.tokenQueryPaths = tokenQueryPaths;
        }
    }
}