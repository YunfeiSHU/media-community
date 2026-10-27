package com.example.userservice.controller;

import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import com.example.userservice.common.ApiResponse;
import com.example.userservice.common.CurrentUser;
import com.example.userservice.common.PageResult;
import com.example.userservice.dto.FollowState;
import com.example.userservice.dto.ProfileResponse;
import com.example.userservice.service.FollowService;

import jakarta.servlet.http.HttpServletRequest;

/**
 * 关注关系接口。
 *
 * <p>路径挂在 {@code /users/{uid}} 下而不是 {@code /profiles}：
 * 写操作必须登录，而 {@code /profiles} 在网关是公开白名单（身份头会被剥离）。
 * 放在 {@code /users/**} 下天然走 JWT 必选路径。
 */
@RestController
@RequestMapping("/api/v1/users")
public class FollowController {

    private final FollowService followService;

    public FollowController(FollowService followService) {
        this.followService = followService;
    }

    /** 关注 POST /api/v1/users/{uid}/follow */
    @PostMapping("/{uid}/follow")
    public ApiResponse<FollowState> follow(@PathVariable("uid") String targetUid, HttpServletRequest request) {
        return ApiResponse.success(followService.follow(CurrentUser.uid(request), targetUid));
    }

    /** 取关 DELETE /api/v1/users/{uid}/follow */
    @DeleteMapping("/{uid}/follow")
    public ApiResponse<FollowState> unfollow(@PathVariable("uid") String targetUid, HttpServletRequest request) {
        return ApiResponse.success(followService.unfollow(CurrentUser.uid(request), targetUid));
    }

    /**
     * 我对某人的关注状态 GET /api/v1/users/{uid}/relationship
     *
     * <p>没有它，前端刷新后无法知道“已经关注了”，按钮会错误地显示成“关注”。
     */
    @GetMapping("/{uid}/relationship")
    public ApiResponse<FollowState> relationship(@PathVariable("uid") String targetUid,
                                                 HttpServletRequest request) {
        return ApiResponse.success(followService.state(CurrentUser.uid(request), targetUid));
    }

    /** 粉丝 GET /api/v1/users/{uid}/followers */
    @GetMapping("/{uid}/followers")
    public ApiResponse<PageResult<ProfileResponse>> followers(
            @PathVariable("uid") String targetUid,
            @RequestParam(name = "page", defaultValue = "1") int page,
            @RequestParam(name = "pageSize", defaultValue = "20") int pageSize) {
        return ApiResponse.success(followService.followers(targetUid, page, pageSize));
    }

    /** 关注列表 GET /api/v1/users/{uid}/following */
    @GetMapping("/{uid}/following")
    public ApiResponse<PageResult<ProfileResponse>> following(
            @PathVariable("uid") String targetUid,
            @RequestParam(name = "page", defaultValue = "1") int page,
            @RequestParam(name = "pageSize", defaultValue = "20") int pageSize) {
        return ApiResponse.success(followService.following(targetUid, page, pageSize));
    }
}
