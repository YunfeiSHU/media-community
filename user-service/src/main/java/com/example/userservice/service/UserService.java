package com.example.userservice.service;

import java.time.LocalDateTime;
import java.util.List;
import java.util.UUID;

import org.springframework.dao.DuplicateKeyException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.example.userservice.common.BizException;
import com.example.userservice.common.ErrorCode;
import com.example.userservice.dto.UserCreateRequest;
import com.example.userservice.dto.UserUpdateRequest;
import com.example.userservice.entity.User;
import com.example.userservice.entity.UserStatus;
import com.example.userservice.repository.UserRepository;

@Service
public class UserService {

    /** 普通用户角色码，与 {@link com.example.userservice.entity.UserRole#USER} 一致 */
    private static final int DEFAULT_ROLE = 1;

    private final UserRepository userRepository;

    public UserService(UserRepository userRepository) {
        this.userRepository = userRepository;
    }

    /**
     * 创建用户档案（按字段入参，RPC 与内部创建共用同一套赋值逻辑）。
     *
     * <p>之前 RPC 侧（{@code UserDubboServiceImpl.createUser}）手写了一遍同样的赋值，
     * 两份代码只在 role 上有差别（RPC 侧设了、本类没设），合并后不会再出现这种偏差。
     */
    @Transactional
    public User createUser(String nickname, String avatar, String gender, String bio) {
        LocalDateTime now = LocalDateTime.now();
        User user = new User();
        user.setUid(UUID.randomUUID().toString().replace("-", ""));
        user.setNickname(nickname);
        user.setAvatar(avatar);
        user.setGender(gender);
        user.setBio(bio);
        // role 必须显式赋值：`user`.role 虽是 NOT NULL DEFAULT 1，
        // 但显式传入 NULL 不会回落到默认值，MySQL 严格模式下会直接报 Column 'role' cannot be null
        user.setRole(DEFAULT_ROLE);
        user.setStatusEnum(UserStatus.NORMAL);
        user.setCreatedAt(now);
        user.setUpdatedAt(now);
        try {
            userRepository.insert(user);
        }
        catch (DuplicateKeyException exception) {
            // 唯一键只有 uid（随机串），碰撞概率可忽略；异常文案按事实修正（原来写的是 username 冲突）
            throw new BizException(ErrorCode.USER_CONFLICT, "用户标识冲突，请重试");
        }
        return user;
    }

    /**
     * 创建用户档案（HTTP/内部 DTO 入口）。
     *
     * <p>当前没有 HTTP 入口：用户一律由 auth-service 经 Dubbo 创建（走 {@link #createUser}）。
     * 保留本重载是为后续管理端接口提供一个可直接复用的入口。
     */
    @Transactional
    public User create(UserCreateRequest request) {
        return createUser(request.nickname(), request.avatar(), request.gender(), request.bio());
    }

    public User getRequired(Long userId) {
        User user = userRepository.findById(userId);
        if (user == null) {
            throw new BizException(ErrorCode.USER_NOT_FOUND, "user not found: " + userId);
        }
        return user;
    }

    public User getRequiredByUid(String uid) {
        User user = userRepository.findByUid(uid);
        if (user == null) {
            throw new BizException(ErrorCode.USER_NOT_FOUND, "user not found: " + uid);
        }
        return user;
    }

    public List<User> getByUids(List<String> uids) {
        return uids.isEmpty() ? List.of() : userRepository.findByUids(uids);
    }

    /**
     * 公开资料：只有状态为 {@code NORMAL} 的用户可见。
     *
     * <p>不存在、已删除、被冻结/禁用都返回同一个 404，调用方无法据此判断账号状态。
     */
    public User getPublicRequiredByUid(String uid) {
        User user = userRepository.findByUid(uid);
        if (user == null || user.getStatusEnum() != UserStatus.NORMAL) {
            // 文案不带 uid 以外的信息，避免泄露“该账号存在但被禁用”
            throw new BizException(ErrorCode.USER_NOT_FOUND, "profile not found: " + uid);
        }
        return user;
    }

    /** 批量公开资料：未知或非 NORMAL 的 uid 直接省略，不抛异常。 */
    public List<User> getPublicByUids(List<String> uids) {
        if (uids.isEmpty()) {
            return List.of();
        }
        return userRepository.findByUids(uids).stream()
                .filter(user -> user.getStatusEnum() == UserStatus.NORMAL)
                .toList();
    }

    @Transactional
    public User updateProfile(Long userId, UserUpdateRequest request) {
        User user = getRequired(userId);
        if (request.nickname() != null) user.setNickname(request.nickname());
        if (request.avatar() != null) user.setAvatar(request.avatar());
        if (request.gender() != null) user.setGender(request.gender());
        if (request.bio() != null) user.setBio(request.bio());
        user.setUpdatedAt(LocalDateTime.now());
        userRepository.updateProfile(user);
        return user;
    }

    @Transactional
    public User updateProfileByUid(String uid, UserUpdateRequest request) {
        return updateProfile(getRequiredByUid(uid).getId(), request);
    }

    /**
     * 按 uid 更新用户状态。
     *
     * <p>返回值里的 {@code updatedAt} 与写进库里的必须同一个时间：
     * 两次 {@code LocalDateTime.now()} 在 Windows 上粒度可达毫秒级，会导致响应与库里对不上。
     */
    @Transactional
    public User updateStatus(Long userId, UserStatus status) {
        User user = getRequired(userId);
        LocalDateTime now = LocalDateTime.now();
        userRepository.updateStatus(userId, status.getCode(), now);
        user.setStatusEnum(status);
        user.setUpdatedAt(now);
        return user;
    }

    @Transactional
    public User updateStatusByUid(String uid, UserStatus status) {
        return updateStatus(getRequiredByUid(uid).getId(), status);
    }
}
