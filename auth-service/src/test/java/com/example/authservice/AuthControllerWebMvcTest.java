package com.example.authservice;

import org.junit.jupiter.api.Test;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.WebMvcTest;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.http.MediaType;
import org.springframework.test.web.servlet.MockMvc;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.example.authservice.common.BizException;
import com.example.authservice.common.ErrorCode;
import com.example.authservice.controller.AuthController;
import com.example.authservice.dto.LoginResponse;
import com.example.authservice.service.AuthService;
import com.example.authservice.service.LoginRateLimitService;
import com.example.authservice.service.VerificationService;

/**
 * AuthController 的 Web 层测试。
 *
 * <p>只关注「HTTP 契约 + 参数校验 + 身份头约定」，业务逻辑由 service 的 mock 承担。
 */
@WebMvcTest(controllers = AuthController.class)
class AuthControllerWebMvcTest {

    private static final String LOGIN_BODY = """
            {
              "type": "email",
              "target": "user@example.com",
              "password": "Abc123456"
            }
            """;

    @Autowired
    private MockMvc mockMvc;

    @MockBean
    private AuthService authService;

    @MockBean
    private LoginRateLimitService loginRateLimitService;

    @MockBean
    private VerificationService verificationService;

    @Test
    void loginReturnsTokenResponse() throws Exception {
        when(authService.login(any())).thenReturn(new LoginResponse(
                "uid-100", "access-token", "refresh-token", 1800, 604800));

        mockMvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(LOGIN_BODY))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.message").value("success"))
                .andExpect(jsonPath("$.data.uid").value("uid-100"))
                .andExpect(jsonPath("$.data.accessToken").value("access-token"))
                .andExpect(jsonPath("$.data.refreshToken").value("refresh-token"))
                .andExpect(jsonPath("$.data.accessTokenExpiresIn").value(1800))
                .andExpect(jsonPath("$.data.refreshTokenExpiresIn").value(604800));

        // 登录前计数、登录成功后清零：限流只统计失败尝试
        verify(loginRateLimitService).checkBeforeLogin(any());
        verify(loginRateLimitService).onLoginSuccess(any());
    }

    @Test
    void loginDoesNotClearRateLimitCounterWhenLoginFails() throws Exception {
        doThrow(new BizException(ErrorCode.BUSINESS_ERROR, "用户名或密码错误"))
                .when(authService).login(any());

        mockMvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(LOGIN_BODY))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value(40000));

        // 失败不得清零，否则暴力破解永远不会触发限流
        verify(loginRateLimitService, org.mockito.Mockito.never()).onLoginSuccess(any());
    }

    @Test
    void loginReturnsRateLimitErrorWhenIpExceedsWindowLimit() throws Exception {
        doThrow(new BizException(ErrorCode.TOO_MANY_REQUESTS, "登录请求过于频繁"))
                .when(loginRateLimitService).checkBeforeLogin(any());

        mockMvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(LOGIN_BODY))
                .andExpect(status().isTooManyRequests())
                .andExpect(jsonPath("$.code").value(42901))
                .andExpect(jsonPath("$.message").value("登录请求过于频繁"))
                .andExpect(jsonPath("$.data").doesNotExist());
    }

    @Test
    void refreshReturnsRotatedTokenResponse() throws Exception {
        when(authService.refresh(any())).thenReturn(new LoginResponse(
                "uid-100", "new-access-token", "new-refresh-token", 1800, 604800));

        mockMvc.perform(post("/api/v1/auth/token/refresh")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "refreshToken": "old-refresh-token"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.accessToken").value("new-access-token"))
                .andExpect(jsonPath("$.data.refreshToken").value("new-refresh-token"));
    }

    @Test
    void logoutReturnsSuccessResponse() throws Exception {
        mockMvc.perform(post("/api/v1/auth/logout")
                        .header("Authorization", "Bearer access-token"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.message").value("success"))
                .andExpect(jsonPath("$.data").doesNotExist());

        verify(authService).logout("Bearer access-token", null);
    }

    @Test
    void logoutWithRefreshTokenAlsoDeletesSession() throws Exception {
        mockMvc.perform(post("/api/v1/auth/logout")
                        .header("Authorization", "Bearer access-token")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {"refreshToken": "uid.session.secret"}
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0));

        verify(authService).logout("Bearer access-token", "uid.session.secret");
    }

    @Test
    void logoutReturnsUnauthorizedWhenAuthorizationHeaderMissing() throws Exception {
        // 缺少必填请求头应返回 40101，而不是被兜底成 500
        mockMvc.perform(post("/api/v1/auth/logout"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value(40101));
    }

    @Test
    void forceLogoutReturnsSuccessResponseForAdmin() throws Exception {
        mockMvc.perform(post("/api/v1/auth/force-logout")
                        .param("uid", "uid-100")
                        .header("X-User-Id", "admin-1")
                        .header("X-User-Role", "ADMIN")
                        .header("Authorization", "Bearer access-token"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.message").value("success"))
                .andExpect(jsonPath("$.data").doesNotExist());

        verify(authService).forceLogout("uid-100", "Bearer access-token");
    }

    @Test
    void forceLogoutRejectedForNormalUser() throws Exception {
        // 角色只认网关注入的 X-User-Role：普通用户 403，且不会落到业务层
        mockMvc.perform(post("/api/v1/auth/force-logout")
                        .param("uid", "uid-100")
                        .header("X-User-Id", "user-1")
                        .header("X-User-Role", "user")
                        .header("Authorization", "Bearer access-token"))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.code").value(40301));

        verify(authService, org.mockito.Mockito.never()).forceLogout(any(), any());
    }

    @Test
    void forceLogoutRejectedWhenNoIdentityHeaderPresent() throws Exception {
        // 直连/未经网关：没有任何身份头 → 401（未认证）
        mockMvc.perform(post("/api/v1/auth/force-logout")
                        .param("uid", "uid-100")
                        .param("role", "ADMIN")
                        .header("Authorization", "Bearer access-token"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value(40101));

        verify(authService, org.mockito.Mockito.never()).forceLogout(any(), any());
    }

    @Test
    void forceLogoutRejectedWhenRoleNotInjectedEvenIfForgedInParams() throws Exception {
        // 有 uid 但网关未注入角色（如白名单路径/直连）：参数里伪造 role=ADMIN 无效 → 403
        mockMvc.perform(post("/api/v1/auth/force-logout")
                        .param("uid", "uid-100")
                        .param("role", "ADMIN")
                        .header("X-User-Id", "uid-100")
                        .header("Authorization", "Bearer access-token"))
                .andExpect(status().isForbidden())
                .andExpect(jsonPath("$.code").value(40301));

        verify(authService, org.mockito.Mockito.never()).forceLogout(any(), any());
    }

    @Test
    void forceLogoutRejectedWhenLegacyUidHeaderForged() throws Exception {
        // 已废弃的非标准头 uid 不再被信任：只带 uid 头 → 401
        mockMvc.perform(post("/api/v1/auth/force-logout")
                        .param("uid", "uid-100")
                        .header("uid", "admin-1")
                        .header("X-User-Role", "ADMIN")
                        .header("Authorization", "Bearer access-token"))
                .andExpect(status().isUnauthorized())
                .andExpect(jsonPath("$.code").value(40101));

        verify(authService, org.mockito.Mockito.never()).forceLogout(any(), any());
    }

    @Test
    void sendVerificationReturnsLearningModeSmsCode() throws Exception {
        when(verificationService.send(any(), any(), eq("13800138000"))).thenReturn("123456");

        mockMvc.perform(post("/api/v1/auth/verification/send")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "purpose": "register",
                                  "type": "phone",
                                  "target": "13800138000"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.type").value("phone"))
                .andExpect(jsonPath("$.data.code").value("123456"));

        verify(verificationService).send(any(), any(), eq("13800138000"));
    }

    @Test
    void sendVerificationOmitsCodeWhenExposeCodeDisabled() throws Exception {
        // 生产模式（auth.verification.expose-code=false）：service 返回 null，响应里不带验证码
        when(verificationService.send(any(), any(), eq("user@example.com"))).thenReturn(null);

        mockMvc.perform(post("/api/v1/auth/verification/send")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "purpose": "register",
                                  "type": "email",
                                  "target": "user@example.com"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.data.type").value("email"))
                .andExpect(jsonPath("$.data.code").isEmpty());

        verify(verificationService).send(any(), any(), eq("user@example.com"));
    }

    @Test
    void malformedRequestBodyReturnsBadRequestInsteadOfServerError() throws Exception {
        // 枚举值非法 → HttpMessageNotReadableException，必须映射为 40001 而不是 50000
        mockMvc.perform(post("/api/v1/auth/verification/send")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "purpose": "register",
                                  "type": "phonex",
                                  "target": "13800138000"
                                }
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value(40001));
    }

    @Test
    void loginValidationRejectsMissingTarget() throws Exception {
        mockMvc.perform(post("/api/v1/auth/login")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "type": "phone",
                                  "password": "Abc123456"
                                }
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value(40001))
                .andExpect(jsonPath("$.message").value("target 不能为空"));
    }

    @Test
    void registerPhoneReturnsSuccess() throws Exception {
        mockMvc.perform(post("/api/v1/auth/register/phone")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "phone": "13800138000",
                                  "code": "123456",
                                  "password": "Abc123456"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.message").value("success"))
                .andExpect(jsonPath("$.data").doesNotExist());

        verify(authService).registerByContact(any(), eq("13800138000"),
                eq("123456"), eq("Abc123456"));
    }

    @Test
    void registerEmailReturnsSuccess() throws Exception {
        mockMvc.perform(post("/api/v1/auth/register/email")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "email": "user@example.com",
                                  "code": "123456",
                                  "password": "Abc123456"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.message").value("success"))
                .andExpect(jsonPath("$.data").doesNotExist());

        verify(authService).registerByContact(any(), eq("user@example.com"),
                eq("123456"), eq("Abc123456"));
    }

    @Test
    void registerPhoneReturnsBusinessErrorWhenContactAlreadyExists() throws Exception {
        doThrow(new BizException(ErrorCode.BUSINESS_ERROR, "联系方式已注册"))
                .when(authService).registerByContact(any(), eq("13800138000"),
                        eq("123456"), eq("Abc123456"));

        mockMvc.perform(post("/api/v1/auth/register/phone")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("""
                                {
                                  "phone": "13800138000",
                                  "code": "123456",
                                  "password": "Abc123456"
                                }
                                """))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.code").value(40000))
                .andExpect(jsonPath("$.message").value("联系方式已注册"))
                .andExpect(jsonPath("$.data").doesNotExist());
    }
}
