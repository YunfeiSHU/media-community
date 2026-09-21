package com.example.gatewayservice.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

/**
 * 网关限流配置，对应 application.yml 中的 gateway.rate-limit.*
 */
@ConfigurationProperties(prefix = "gateway.rate-limit")
public class GatewayRateLimitProperties {

    private GlobalLimit globalLimit = new GlobalLimit();
    private IpLimit ipLimit = new IpLimit();

    public GlobalLimit getGlobalLimit() {
        return globalLimit;
    }

    public void setGlobalLimit(GlobalLimit globalLimit) {
        this.globalLimit = globalLimit;
    }

    public IpLimit getIpLimit() {
        return ipLimit;
    }

    public void setIpLimit(IpLimit ipLimit) {
        this.ipLimit = ipLimit;
    }

    public static class GlobalLimit {
        /** Redis key 前缀 */
        private String keyPrefix = "gateway:rate:global";

        /** 每秒请求数 (tokens per second) */
        private Integer replenishRate = 10000;

        /** 容量 (max tokens) */
        private Integer burstCapacity = 10000;

        public String getKeyPrefix() {
            return keyPrefix;
        }

        public void setKeyPrefix(String keyPrefix) {
            this.keyPrefix = keyPrefix;
        }

        public Integer getReplenishRate() {
            return replenishRate;
        }

        public void setReplenishRate(Integer replenishRate) {
            this.replenishRate = replenishRate;
        }

        public Integer getBurstCapacity() {
            return burstCapacity;
        }

        public void setBurstCapacity(Integer burstCapacity) {
            this.burstCapacity = burstCapacity;
        }
    }

    public static class IpLimit {
        /** Redis key 前缀 */
        private String keyPrefix = "gateway:rate:ip";

        /** 每秒请求数 (tokens per second) */
        private Integer replenishRate = 100;

        /** 容量 (max tokens) */
        private Integer burstCapacity = 100;

        public String getKeyPrefix() {
            return keyPrefix;
        }

        public void setKeyPrefix(String keyPrefix) {
            this.keyPrefix = keyPrefix;
        }

        public Integer getReplenishRate() {
            return replenishRate;
        }

        public void setReplenishRate(Integer replenishRate) {
            this.replenishRate = replenishRate;
        }

        public Integer getBurstCapacity() {
            return burstCapacity;
        }

        public void setBurstCapacity(Integer burstCapacity) {
            this.burstCapacity = burstCapacity;
        }
    }
}
