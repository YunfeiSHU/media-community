package com.example.gatewayservice.config;

import java.util.ArrayList;
import java.util.List;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * 网关认证相关配置，对应 application.yml 中的 gateway.auth.*
 */
@ConfigurationProperties(prefix = "gateway.auth")
public class GatewayAuthProperties {

    /** 无需认证即可访问的路径，Ant 风格 */
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

    public static class Jwt {
        /** 签名密钥，HS256 要求至少 32 字节 */
        private String secret;

        /** 用户 ID 所在的 claim 名 */
        private String userIdClaim = "sub";

        /** 角色所在的 claim 名 */
        private String roleClaim = "role";

        public String getSecret() {
            return secret;
        }

        public void setSecret(String secret) {
            this.secret = secret;
        }

        public String getUserIdClaim() {
            return userIdClaim;
        }

        public void setUserIdClaim(String userIdClaim) {
            this.userIdClaim = userIdClaim;
        }

        public String getRoleClaim() {
            return roleClaim;
        }

        public void setRoleClaim(String roleClaim) {
            this.roleClaim = roleClaim;
        }
    }
}