package com.example.userservice.dto;

import java.time.LocalDateTime;
import java.util.Map;

import com.example.userservice.entity.User;

/**
 * 公开用户资料（对客视图）。
 *
 * <p>与 {@link UserResponse} 的区别：这里的字段**全部对外公开**，
 * 因此刻意不包含 {@code userId}（内部自增主键）、{@code status}、手机号 / 邮箱。
 * 前端首页卡片、播放页作者卡、直播间主播信息都用这个结构。
 *
 * <p>{@code videoCount} 目前恒为 0：视频数在 video-service，本服务不跨服务查询；
 * 等 BFF 或批量接口就绪后再补。{@code followerCount / followingCount} 已由
 * user_follow 表实时统计（S4）。
 */
public record ProfileResponse(
        String uid,
        String nickname,
        String avatar,
        String bio,
        String gender,
        long followerCount,
        long followingCount,
        long videoCount,
        LocalDateTime createdAt
) {
    public static ProfileResponse from(User user) {
        return from(user, 0L, 0L);
    }

    public static ProfileResponse from(User user, long followerCount, long followingCount) {
        return new ProfileResponse(
                user.getUid(),
                user.getNickname(),
                user.getAvatar(),
                user.getBio(),
                user.getGender(),
                followerCount,
                followingCount,
                0L,
                user.getCreatedAt());
    }

    /** 关注列表用：行里只有公开字段，计数从批量统计结果里取。 */
    public static ProfileResponse from(FollowRow row,
                                       Map<String, Long> followerCounts,
                                       Map<String, Long> followingCounts) {
        return new ProfileResponse(
                row.getUid(),
                row.getNickname(),
                row.getAvatar(),
                row.getBio(),
                row.getGender(),
                followerCounts.getOrDefault(row.getUid(), 0L),
                followingCounts.getOrDefault(row.getUid(), 0L),
                0L,
                row.getCreatedAt());
    }
}
