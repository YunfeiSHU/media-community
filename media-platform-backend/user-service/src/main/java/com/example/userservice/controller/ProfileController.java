package com.example.userservice.controller;

import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import com.example.userservice.common.ApiResponse;
import com.example.userservice.common.BizException;
import com.example.userservice.common.ErrorCode;
import com.example.userservice.dto.ProfileListResponse;
import com.example.userservice.dto.ProfileResponse;
import com.example.userservice.service.FollowService;
import com.example.userservice.service.UserService;

/**
 * 公开用户资料接口（无需登录）。
 *
 * <p>为什么单独开 {@code /profiles} 而不是 {@code GET /users/{uid}}：
 * 网关白名单用 Ant 匹配，若把 {@code /api/v1/users/*} 放开，会连 {@code /users/me} 一起放行，
 * 语义上把“公开资料”和“我的资料”混在一起，后续很容易误判。独立前缀让边界一眼可见。
 */
@RestController
@RequestMapping("/api/v1/profiles")
public class ProfileController {

    /** 批量查询上限：超过直接 40001，避免一次拖出全表 */
    private static final int MAX_BATCH = 100;

    private final UserService userService;

    private final FollowService followService;

    public ProfileController(UserService userService, FollowService followService) {
        this.userService = userService;
        this.followService = followService;
    }

    /**
     * 按 uid 查公开资料。
     *
     * <p>用户不存在、或状态不是 {@code NORMAL}（已删除/冻结/禁用）统一返回 404，
     * 不区分“不存在”与“不可见”，避免用枚举探测账号状态。
     */
    @GetMapping("/{uid}")
    public ApiResponse<ProfileResponse> get(@PathVariable String uid) {
        var user = userService.getPublicRequiredByUid(uid);
        var counts = followService.followerCounts(List.of(uid));
        var following = followService.followingCounts(List.of(uid));
        return ApiResponse.success(ProfileResponse.from(user,
                counts.getOrDefault(uid, 0L), following.getOrDefault(uid, 0L)));
    }

    /**
     * 批量查公开资料：{@code GET /api/v1/profiles?uids=1001,1002}。
     *
     * <p>未知 uid 直接省略（不报错），返回顺序与入参顺序无关；
     * 这是首页卡片“一次请求拿一屏作者”的关键接口，避免每个卡片发一次请求。
     */
    @GetMapping
    public ApiResponse<ProfileListResponse> batch(
            @RequestParam(name = "uids", required = false) String uids) {
        List<String> parsed = parseUids(uids);
        var users = userService.getPublicByUids(parsed);
        List<String> found = users.stream().map(user -> user.getUid()).toList();
        var followerCounts = followService.followerCounts(found);
        var followingCounts = followService.followingCounts(found);

        List<ProfileResponse> items = new ArrayList<>(users.size());
        for (var user : users) {
            items.add(ProfileResponse.from(user,
                    followerCounts.getOrDefault(user.getUid(), 0L),
                    followingCounts.getOrDefault(user.getUid(), 0L)));
        }
        return ApiResponse.success(new ProfileListResponse(items));
    }

    private List<String> parseUids(String raw) {
        if (raw == null || raw.isBlank()) {
            return List.of();
        }
        Set<String> unique = new LinkedHashSet<>();
        for (String part : raw.split(",")) {
            String uid = part.trim();
            if (!uid.isEmpty()) {
                unique.add(uid);
            }
        }
        if (unique.size() > MAX_BATCH) {
            throw new BizException(ErrorCode.INVALID_PARAM,
                    "uids 最多 " + MAX_BATCH + " 个");
        }
        return List.copyOf(unique);
    }
}
