package com.example.product.common;

import java.security.SecureRandom;
import java.util.Locale;
import java.util.concurrent.atomic.AtomicLong;

/**
 * 业务编号生成器（product_id / sku_id）。
 *
 * <p>结构：{@code 前缀 + 时间戳(36进制, 含进程内自增位) + 随机串}，长度 &lt;= 24，满足 VARCHAR(32)。
 * 单机唯一由「毫秒 + 12bit 自增 + 随机串」共同保证，DB 侧仍有唯一索引兜底。
 */
public final class IdGenerator {

    private static final char PREFIX_PRODUCT = 'P';

    private static final char PREFIX_SKU = 'S';

    private static final int SEQUENCE_MASK = 0xFFF;

    private static final AtomicLong SEQUENCE = new AtomicLong();

    private static final SecureRandom RANDOM = new SecureRandom();

    private IdGenerator() {
    }

    public static String nextProductId() {
        return PREFIX_PRODUCT + nextSuffix();
    }

    public static String nextSkuId() {
        return PREFIX_SKU + nextSuffix();
    }

    private static String nextSuffix() {
        long millis = System.currentTimeMillis();
        long sequence = SEQUENCE.getAndIncrement() & SEQUENCE_MASK;
        String timePart = Long.toString((millis << 12) | sequence, 36).toUpperCase(Locale.ROOT);
        String randomPart = Long.toString(Math.abs(RANDOM.nextLong()), 36)
                .toUpperCase(Locale.ROOT);
        return timePart + randomPart.substring(0, 6);
    }
}
