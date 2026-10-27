package com.example.authservice.dto;

/**
 * 登出请求体。
 *
 * <p>{@code refreshToken} 可选：带上则同时删除该刷新令牌对应的会话，
 * 让“退出登录”真正失效而不只是拉黑当前 access token；
 * 不传则退化为旧行为（仅拉黑 access token），兼容老客户端。
 */
public record LogoutRequest(String refreshToken) {
}
