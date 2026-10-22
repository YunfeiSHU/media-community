package com.example.authservice.service;

import java.time.LocalDateTime;
import java.util.Locale;
import java.util.Objects;
import java.util.UUID;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.example.authservice.common.BizException;
import com.example.authservice.common.ErrorCode;
import com.example.authservice.dto.LoginRequest;
import com.example.authservice.dto.LoginResponse;
import com.example.authservice.dto.RefreshTokenRequest;
import com.example.authservice.entity.Account;
import com.example.authservice.repository.AccountRepository;
import com.example.authservice.verification.enums.VerificationPurpose;
import com.example.authservice.verification.enums.VerificationType;
import com.example.userservice.rpc.RpcUser;

@Service
public class AuthService {

    /** 普通用户角色码，与 user-service 的 {@code UserRole.USER}（1）保持一致 */
    private static final int USER_ROLE_CODE = 1;

    private final AccountRepository userRepository;
    private final PasswordEncoder passwordEncoder;
    private final VerificationService verificationService;
    private final JwtTokenService jwtTokenService;
    private final SessionService sessionService;
    private final JwtBlacklistService jwtBlacklistService;
    private final UserServiceClient userServiceClient;
    private final long accessTokenTtlSeconds;
    private final long refreshTokenTtlSeconds;

    public AuthService(AccountRepository userRepository, PasswordEncoder passwordEncoder,
                       VerificationService verificationService, JwtTokenService jwtTokenService,
                       SessionService sessionService,
                       JwtBlacklistService jwtBlacklistService,
                       UserServiceClient userServiceClient,
                       @Value("${auth.jwt.access-token-ttl-seconds}") long accessTokenTtlSeconds,
                       @Value("${auth.jwt.refresh-token-ttl-seconds}") long refreshTokenTtlSeconds) {
        this.userRepository = userRepository;
        this.passwordEncoder = passwordEncoder;
        this.verificationService = verificationService;
        this.jwtTokenService = jwtTokenService;
        this.sessionService = sessionService;
        this.jwtBlacklistService = jwtBlacklistService;
        this.userServiceClient = userServiceClient;
        this.accessTokenTtlSeconds = accessTokenTtlSeconds;
        this.refreshTokenTtlSeconds = refreshTokenTtlSeconds;
    }

    /**
     * 登录。
     *
     * <p>uid 与角色以 user-service 的用户档案为准（Dubbo 调用）；
     * 档案缺失时退回 account 表的 uid 与 {@code USER} 角色（RPC 强依赖已写入技术方案，此处不再保留“无 RPC”降级分支）。
     */
    public LoginResponse login(LoginRequest request) {
        Account user = userRepository.findByContact(normalizeTarget(request.target()));
        if (user == null || !Integer.valueOf(1).equals(user.getStatus())
                || !passwordEncoder.matches(request.password(), user.getPasswordHash())) {
            throw new BizException(ErrorCode.BUSINESS_ERROR, "用户名或密码错误");
        }
        String jti = UUID.randomUUID().toString();
        RpcUser rpcUser = userServiceClient.getUser(user.getUid());
        String uid = rpcUser != null && rpcUser.getUid() != null ? rpcUser.getUid() : user.getUid();
        int roleCode = Objects.requireNonNullElse(rpcUser == null ? null : rpcUser.getRole(), USER_ROLE_CODE);
        String accessToken = jwtTokenService.generateAccessToken(uid, jti, roleCode, accessTokenTtlSeconds);
        SessionService.Session session = sessionService.create(uid, jti);
        return new LoginResponse(uid, accessToken, session.refreshToken(),
                accessTokenTtlSeconds, refreshTokenTtlSeconds);
    }

    public LoginResponse refresh(RefreshTokenRequest request) {
        String uid = sessionService.uidFrom(request.refreshToken());
        String jti = UUID.randomUUID().toString();
        SessionService.Session session = sessionService.rotate(request.refreshToken(), jti);
        RpcUser rpcUser = userServiceClient.getUser(uid);
        int roleCode = Objects.requireNonNullElse(rpcUser == null ? null : rpcUser.getRole(), USER_ROLE_CODE);
        String accessToken = jwtTokenService.generateAccessToken(uid, jti, roleCode, accessTokenTtlSeconds);
        return new LoginResponse(uid, accessToken, session.refreshToken(),
                accessTokenTtlSeconds, refreshTokenTtlSeconds);
    }

    /**
     * 退出登录：拉黑当前 Access Token（按 jti 写入 Redis，TTL = token 剩余寿命）。
     *
     * <p><b>注意：本方法不会失效 Refresh Token。</b>因为 Access Token 里只有 jti、没有 sessionId，
     * 要“只失效当前设备”就需要维护 jti → sessionId 的反查索引；目前的取舍是登出仅提前作废当前 Access Token，
     * 需要真正断开用户（全部设备）请用 {@link #forceLogout}。
     *
     * @param authorization 形如 {@code Bearer <token>}
     */
    public void logout(String authorization) {
        jwtBlacklistService.blacklist(authorization);
    }

    /**
     * 强制下线：清掉目标用户的所有会话并拉黑当前 Access Token。
     *
     * <p>调用者的管理员身份由 {@code AuthController} 依据网关注入的 {@code X-User-Role}
     * 校验（角色只能来自 JWT），此处只负责执行。
     *
     * @param uid 被下线的目标用户 uid（来自请求参数，不是调用者身份）
     */
    public void forceLogout(String uid, String authorization) {
        sessionService.deleteByUid(uid);
        jwtBlacklistService.blacklist(authorization);
    }

    /**
     * 按手机号 / 邮箱注册。
     *
     * <p>顺序：校验并消费验证码 → 查重 → 在 user-service 建用户档案 → 写 account。
     *
     * <p>已知取舍（本地事务包住了远程调用与 Redis 操作）：
     * <ul>
     *     <li>验证码在事务内被删除，事务回滚时 Redis 不会回滚，用户需重新获取验证码（可接受）；</li>
     *     <li>user-service 建档案成功、account 写入失败时会留下无 account 的孤儿 user 行（无补偿）。</li>
     * </ul>
     */
    @Transactional
    public void registerByContact(VerificationType type, String target, String code, String password) {
        verificationService.verify(VerificationPurpose.REGISTER, type, target, code);
        String normalizedTarget = normalizeTarget(target);
        if (userRepository.findByContact(normalizedTarget) != null) {
            throw new BizException(ErrorCode.BUSINESS_ERROR, "联系方式已注册");
        }

        RpcUser createdUser = userServiceClient.createUser();
        if (createdUser == null || createdUser.getUid() == null || createdUser.getUid().isBlank()) {
            // 不能“自己造一个 uid”：那样 account 与 user 表会对不上，用户能登录但 GET /users/me 永远 404
            throw new BizException(ErrorCode.INTERNAL_ERROR, "创建用户档案失败，请稍后重试");
        }
        LocalDateTime now = LocalDateTime.now();
        Account user = new Account();
        user.setUid(createdUser.getUid());
        user.setPhone(type == VerificationType.PHONE ? normalizedTarget : null);
        user.setEmail(type == VerificationType.EMAIL ? normalizedTarget : null);
        user.setPasswordHash(passwordEncoder.encode(password));
        user.setStatus(1);
        user.setCreatedAt(now);
        user.setUpdatedAt(now);

        try {
            userRepository.insert(user);
        } catch (DuplicateKeyException exception) {
            throw new BizException(ErrorCode.BUSINESS_ERROR, "联系方式已注册");
        }
    }

    /**
     * 归一化联系方式。
     *
     * <p><b>不</b>根据声明的 {@code type} 决定是否转小写，而是看目标本身是否像邮箱：
     * 底层查询是 {@code WHERE phone = ? OR email = ?}（见 {@code AccountRepository#findByContact}），
     * 两个字段都会参与匹配；若按 type 判断，{@code type=phone} 却传了邮箱时就不会转小写，
     * 导致查不到用大写邮箱注册的账号（登录报“用户名或密码错误”）。
     */
    private String normalizeTarget(String target) {
        String trimmed = target == null ? "" : target.trim();
        return trimmed.contains("@") ? trimmed.toLowerCase(Locale.ROOT) : trimmed;
    }
}
