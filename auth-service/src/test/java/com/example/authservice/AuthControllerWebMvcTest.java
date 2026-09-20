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

import com.example.authservice.controller.AuthController;
import com.example.authservice.dto.LoginResponse;
import com.example.authservice.exception.BusinessException;
import com.example.authservice.service.AuthService;
import com.example.authservice.service.LoginRateLimitService;
import com.example.authservice.service.VerificationService;

@WebMvcTest(AuthController.class)
class AuthControllerWebMvcTest {

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
                        .content("""
                                {
                                  "type": "email",
                                  "target": "user@example.com",
                                  "password": "Abc123456"
                                }
                                """))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.code").value(0))
                .andExpect(jsonPath("$.message").value("success"))
                .andExpect(jsonPath("$.data.uid").value("uid-100"))
                .andExpect(jsonPath("$.data.accessToken").value("access-token"))
                .andExpect(jsonPath("$.data.refreshToken").value("refresh-token"))
                .andExpect(jsonPath("$.data.accessTokenExpiresIn").value(1800))
                .andExpect(jsonPath("$.data.refreshTokenExpiresIn").value(604800));

        verify(loginRateLimitService).check(any());
    }

                @Test
                void loginReturnsRateLimitErrorWhenIpExceedsWindowLimit() throws Exception {
                                doThrow(new BusinessException(1003, "登录请求过于频繁"))
                                                                .when(loginRateLimitService).check(any());

                                mockMvc.perform(post("/api/v1/auth/login")
                                                                                                .contentType(MediaType.APPLICATION_JSON)
                                                                                                .content("""
                                                                                                                                {
                                                                                                                                        "type": "email",
                                                                                                                                        "target": "user@example.com",
                                                                                                                                        "password": "Abc123456"
                                                                                                                                }
                                                                                                                                """))
                                                                .andExpect(status().isBadRequest())
                                                                .andExpect(jsonPath("$.code").value(1003))
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

                verify(authService).logout("Bearer access-token");
        }

        @Test
        void forceLogoutReturnsSuccessResponse() throws Exception {
                mockMvc.perform(post("/api/v1/auth/force-logout")
                                                .param("uid", "uid-100")
                                                .header("Authorization", "Bearer access-token"))
                                .andExpect(status().isOk())
                                .andExpect(jsonPath("$.code").value(0))
                                .andExpect(jsonPath("$.message").value("success"))
                                .andExpect(jsonPath("$.data").doesNotExist());

                verify(authService).forceLogout("uid-100", "Bearer access-token");
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
    void sendVerificationReturnsNullCodeForEmail() throws Exception {
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
                .andExpect(jsonPath("$.code").value(1002))
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
        doThrow(new BusinessException(1001, "联系方式已注册"))
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
                .andExpect(jsonPath("$.code").value(1001))
                .andExpect(jsonPath("$.message").value("联系方式已注册"))
                .andExpect(jsonPath("$.data").doesNotExist());
    }
}
