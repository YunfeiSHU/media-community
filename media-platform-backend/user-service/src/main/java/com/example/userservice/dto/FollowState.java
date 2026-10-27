package com.example.userservice.dto;

/**
 * 关注 / 取关后的状态。
 *
 * @param following     当前是否已关注
 * @param followerCount 目标用户当前的粉丝数（用于前端就地更新，不必再拉一次资料）
 */
public record FollowState(boolean following, long followerCount) {
}
