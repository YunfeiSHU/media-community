package com.example.userservice.dto;

import java.util.List;

/**
 * 批量公开资料响应。
 *
 * <p>用 {@code {items:[...]}} 而不是直接返回数组：与视频/直播列表保持同一套包裹风格，
 * 后续加 {@code total} 等字段时不会破坏前端解析。
 */
public record ProfileListResponse(List<ProfileResponse> items) {
}
