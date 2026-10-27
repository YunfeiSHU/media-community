package com.example.order.common;

import java.security.SecureRandom;
import java.util.Locale;
import java.util.concurrent.atomic.AtomicLong;

/**
 * 业务编号生成器（order_id / payment_id）。
 *
 * <p>结构：{@code 前缀 + 时间戳(36进制, 含进程内自增位) + 随机串}，长度 &lt;= 24，满足 VARCHAR(32)。
 * 单机唯一由「毫秒 + 12bit 自增 + 随机串」共同保证，DB 侧仍有唯一索引兜底。
 */
public final class IdGenerator {

    private static final char PREFIX_ORDER = 'O';

    private static final char PREFIX_PAYMENT = 'P';

    private static final int SEQUENCE_MASK = 0xFFF;

    /** 随机位字符数 */
    private static final int RANDOM_CHARS = 6;

    /** 36^6，保证随机部分最多 RANDOM_CHARS 位 */
    private static final long RANDOM_BOUND = 2_176_782_336L;

    private static final AtomicLong SEQUENCE = new AtomicLong();

    private static final SecureRandom RANDOM = new SecureRandom();

    private IdGenerator() {
    }

    public static String nextOrderId() {
        return PREFIX_ORDER + nextSuffix();
    }

    public static String nextPaymentId() {
        return PREFIX_PAYMENT + nextSuffix();
    }

    /**
     * 拼接时间部分与随机部分。
     *
     * <p>随机部分用 {@link Math#floorMod} 而非 {@code Math.abs}：
     * <ul>
     *     <li>{@code nextLong()} 可能返回 {@code Long.MIN_VALUE}，{@code Math.abs} 会溢出成负数，
     *         进制转换后得到带负号的串（随机位不足 6 位）；</li>
     *     <li>绝对值很小时（&lt; 36^5）进制串甚至不足 6 位，{@code substring(0, 6)} 会直接抛
     *         {@code StringIndexOutOfBoundsException}（概率约 1e-11，但失败方式是 500）。</li>
     * </ul>
     * 现在改为先取模到 [0, 36^6) 再左侧补零，长度恒定。
     */
    private static String nextSuffix() {
        long millis = System.currentTimeMillis();
        long sequence = SEQUENCE.getAndIncrement() & SEQUENCE_MASK;
        String timePart = Long.toString((millis << 12) | sequence, 36).toUpperCase(Locale.ROOT);
        String randomPart = Long.toString(Math.floorMod(RANDOM.nextLong(), RANDOM_BOUND), 36)
                .toUpperCase(Locale.ROOT);
        return timePart + "0".repeat(RANDOM_CHARS - randomPart.length()) + randomPart;
    }
}
