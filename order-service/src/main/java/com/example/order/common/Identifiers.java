package com.example.order.common;

import java.util.regex.Pattern;

/**
 * 标识符工具。
 *
 * <p>HTTP 路径中的 {@code {id}} 同时支持两种形态：
 * <ul>
 *     <li>纯数字：表内部主键 {@code id}（BIGINT）</li>
 *     <li>其它：业务编号 {@code order_id}（VARCHAR(32)）</li>
 * </ul>
 */
public final class Identifiers {

    private static final Pattern SNOWFLAKE_LIKE_NUMBER = Pattern.compile("^\\d{1,19}$");

    private Identifiers() {
    }

    public static boolean isInternalId(String rawId) {
        return rawId != null && SNOWFLAKE_LIKE_NUMBER.matcher(rawId).matches();
    }

    public static Long parseInternalId(String rawId) {
        return isInternalId(rawId) ? Long.valueOf(rawId) : null;
    }
}
