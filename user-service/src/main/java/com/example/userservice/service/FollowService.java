package com.example.userservice.service;

import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.example.userservice.common.BizException;
import com.example.userservice.common.ErrorCode;
import com.example.userservice.common.PageResult;
import com.example.userservice.dto.FollowRow;
import com.example.userservice.dto.FollowState;
import com.example.userservice.dto.ProfileResponse;
import com.example.userservice.dto.UidCount;
import com.example.userservice.repository.FollowRepository;
import com.example.userservice.repository.UserRepository;

/**
 * 关注关系服务（S4）。
 *
 * <p>关注落在 user-service 而不是 video-service：它是用户域的关系，
 * 与视频无关；跨服务调用会把「看一个视频」变成「依赖两个服务都活着」。
 */
@Service
public class FollowService {

    private static final int MAX_PAGE_SIZE = 100;

    private final FollowRepository followRepository;

    private final UserRepository userRepository;

    public FollowService(FollowRepository followRepository, UserRepository userRepository) {
        this.followRepository = followRepository;
        this.userRepository = userRepository;
    }

    /** 关注：不能关注自己，目标必须存在；重复关注幂等。 */
    @Transactional
    public FollowState follow(String uid, String targetUid) {
        String me = requireUid(uid);
        String target = requireTarget(targetUid);
        if (me.equals(target)) {
            throw new BizException(ErrorCode.INVALID_PARAM, "不能关注自己");
        }
        if (userRepository.findByUid(target) == null) {
            throw new BizException(ErrorCode.USER_NOT_FOUND, "用户不存在: " + target);
        }
        followRepository.insert(me, target, LocalDateTime.now());
        return new FollowState(true, followRepository.countFollowers(target));
    }

    /** 取关：未关注时也返回成功（幂等）。 */
    @Transactional
    public FollowState unfollow(String uid, String targetUid) {
        String me = requireUid(uid);
        String target = requireTarget(targetUid);
        followRepository.delete(me, target);
        return new FollowState(false, followRepository.countFollowers(target));
    }

    /** 当前用户对目标用户的关注状态（初始化按钮用）。 */
    public FollowState state(String uid, String targetUid) {
        String me = requireUid(uid);
        String target = requireTarget(targetUid);
        boolean following = followRepository.countRelation(me, target) > 0;
        return new FollowState(following, followRepository.countFollowers(target));
    }

    /** 粉丝列表。 */
    public PageResult<ProfileResponse> followers(String targetUid, int page, int pageSize) {
        return pageOf(targetUid, page, pageSize, true);
    }

    /** 关注列表。 */
    public PageResult<ProfileResponse> following(String targetUid, int page, int pageSize) {
        return pageOf(targetUid, page, pageSize, false);
    }

    private PageResult<ProfileResponse> pageOf(String targetUid, int page, int pageSize, boolean followers) {
        String target = requireTarget(targetUid);
        if (page < 1) {
            throw new BizException(ErrorCode.INVALID_PARAM, "page 必须从 1 开始");
        }
        if (pageSize < 1 || pageSize > MAX_PAGE_SIZE) {
            throw new BizException(ErrorCode.INVALID_PARAM, "pageSize 必须在 1~" + MAX_PAGE_SIZE + " 之间");
        }

        long total = followers
                ? followRepository.countFollowers(target)
                : followRepository.countFollowing(target);
        if (total == 0) {
            return PageResult.empty(page, pageSize);
        }

        int offset = (page - 1) * pageSize;
        List<FollowRow> rows = followers
                ? followRepository.pageFollowers(target, offset, pageSize)
                : followRepository.pageFollowing(target, offset, pageSize);
        if (rows.isEmpty()) {
            return PageResult.of(List.of(), total, page, pageSize);
        }

        List<String> uids = rows.stream().map(FollowRow::getUid).toList();
        Map<String, Long> followerCounts = countsByUid(followRepository.countFollowersByUids(uids));
        Map<String, Long> followingCounts = countsByUid(followRepository.countFollowingByUids(uids));

        List<ProfileResponse> items = new ArrayList<>(rows.size());
        for (FollowRow row : rows) {
            items.add(ProfileResponse.from(row, followerCounts, followingCounts));
        }
        return PageResult.of(items, total, page, pageSize);
    }

    /** uid -> count 的映射，缺失按 0 处理。 */
    public static Map<String, Long> countsByUid(List<UidCount> rows) {
        return rows.stream().collect(Collectors.toMap(
                UidCount::getUid,
                row -> row.getCount() == null ? 0L : row.getCount(),
                (left, right) -> left));
    }

    /** 供其它服务复用：批量取关注计数。 */
    public Map<String, Long> followerCounts(List<String> uids) {
        if (uids.isEmpty()) {
            return Map.of();
        }
        return countsByUid(followRepository.countFollowersByUids(uids));
    }

    public Map<String, Long> followingCounts(List<String> uids) {
        if (uids.isEmpty()) {
            return Map.of();
        }
        return countsByUid(followRepository.countFollowingByUids(uids));
    }

    private String requireUid(String uid) {
        if (uid == null || uid.isBlank()) {
            throw new BizException(ErrorCode.UNAUTHORIZED, "缺少用户身份信息，请通过网关登录后访问");
        }
        return uid.trim();
    }

    private String requireTarget(String targetUid) {
        if (targetUid == null || targetUid.isBlank()) {
            throw new BizException(ErrorCode.INVALID_PARAM, "uid 不能为空");
        }
        return targetUid.trim();
    }
}
