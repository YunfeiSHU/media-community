package com.example.authservice.service;

import java.time.LocalDateTime;
import java.util.Objects;
import java.util.UUID;

import org.springframework.beans.factory.annotation.Autowired;
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

    private final AccountRepository userRepository;
    private final PasswordEncoder passwordEncoder;
    private final VerificationService verificationService;
    private final JwtTokenService jwtTokenService;
    private final SessionService sessionService;
    private final JwtBlacklistService jwtBlacklistService;
    private final UserServiceClient userServiceClient;
    private final long accessTokenTtlSeconds;
    private final long refreshTokenTtlSeconds;

    @Autowired
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

    public AuthService(AccountRepository userRepository, PasswordEncoder passwordEncoder,
                       VerificationService verificationService, JwtTokenService jwtTokenService,
                       SessionService sessionService, JwtBlacklistService jwtBlacklistService,
                       long accessTokenTtlSeconds, long refreshTokenTtlSeconds) {
        this(userRepository, passwordEncoder, verificationService, jwtTokenService, sessionService,
                jwtBlacklistService, null, accessTokenTtlSeconds, refreshTokenTtlSeconds);
    }

    public LoginResponse login(LoginRequest request) {
        Account user = userRepository.findByContact(normalizeTarget(request.type(), request.target()));
        if (user == null || !Integer.valueOf(1).equals(user.getStatus())
                || !passwordEncoder.matches(request.password(), user.getPasswordHash())) {
            throw new BizException(ErrorCode.BUSINESS_ERROR, "用户名或密码错误");
        }
        String jti = UUID.randomUUID().toString();
        RpcUser rpcUser = userServiceClient == null ? null : userServiceClient.getUser(user.getUid());
        String uid = rpcUser != null ? rpcUser.getUid() : user.getUid();
        int roleCode = Objects.requireNonNullElse(rpcUser != null ? rpcUser.getRole() : null, 1);
        String accessToken = jwtTokenService.generateAccessToken(uid, jti, roleCode, accessTokenTtlSeconds);
        SessionService.Session session = sessionService.create(uid, jti);
        return new LoginResponse(uid, accessToken, session.refreshToken(),
                accessTokenTtlSeconds, refreshTokenTtlSeconds);
    }

    public LoginResponse refresh(RefreshTokenRequest request) {
        String uid = sessionService.uidFrom(request.refreshToken());
        String jti = UUID.randomUUID().toString();
        SessionService.Session session = sessionService.rotate(request.refreshToken(), jti);
        RpcUser rpcUser = userServiceClient == null ? null : userServiceClient.getUser(uid);
        int roleCode = Objects.requireNonNullElse(rpcUser != null ? rpcUser.getRole() : null, 1);
        String accessToken = jwtTokenService.generateAccessToken(uid, jti, roleCode, accessTokenTtlSeconds);
        return new LoginResponse(uid, accessToken, session.refreshToken(),
                accessTokenTtlSeconds, refreshTokenTtlSeconds);
    }

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

    @Transactional
    public void registerByContact(VerificationType type, String target, String code, String password) {
        verificationService.verify(VerificationPurpose.REGISTER, type, target, code);
        String normalizedTarget = normalizeTarget(type, target);
        if (userRepository.findByContact(normalizedTarget) != null) {
            throw new BizException(ErrorCode.BUSINESS_ERROR, "联系方式已注册");
        }

        RpcUser createdUser = userServiceClient == null ? null : userServiceClient.createUser();
        LocalDateTime now = LocalDateTime.now();
        Account user = new Account();
        if (createdUser != null) {
            user.setUid(createdUser.getUid());
        } else {
            user.setUid(UUID.randomUUID().toString().replace("-", ""));
        }
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

    private String normalizeTarget(VerificationType type, String target) {
        return type == VerificationType.EMAIL ? target.trim().toLowerCase() : target.trim();
    }
}
