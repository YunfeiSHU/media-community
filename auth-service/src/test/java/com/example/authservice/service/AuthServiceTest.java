package com.example.authservice.service;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import org.mockito.Mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.security.crypto.password.PasswordEncoder;

import com.example.authservice.dto.LoginRequest;
import com.example.authservice.dto.LoginResponse;
import com.example.authservice.dto.RefreshTokenRequest;
import com.example.authservice.entity.User;
import com.example.authservice.exception.BusinessException;
import com.example.authservice.repository.UserRepository;
import com.example.authservice.verification.enums.VerificationType;

@ExtendWith(MockitoExtension.class)
class AuthServiceTest {

    @Mock
    private UserRepository userRepository;

    @Mock
    private PasswordEncoder passwordEncoder;

    @Mock
    private VerificationService verificationService;

    @Mock
    private JwtTokenService jwtTokenService;

    @Mock
    private SessionService sessionService;

    @Mock
    private JwtBlacklistService jwtBlacklistService;

    private AuthService authService;

    @BeforeEach
    void setUp() {
        authService = new AuthService(userRepository, passwordEncoder, verificationService,
            jwtTokenService, sessionService, jwtBlacklistService, 1800, 604800);
    }

    @Test
    void loginReturnsAccessAndRefreshTokensForValidUser() {
        User user = enabledUser("uid-100", "user@example.com");
        when(userRepository.findByContact("user@example.com")).thenReturn(user);
        when(passwordEncoder.matches("Abc123456", "encoded-password")).thenReturn(true);
        when(jwtTokenService.generateAccessToken(eq("uid-100"), any(String.class), eq(1800L)))
            .thenReturn("access-token");
        when(sessionService.create(eq("uid-100"), anyString()))
                .thenReturn(new SessionService.Session("refresh-token", 604800));

        LoginResponse response = authService.login(new LoginRequest(
                VerificationType.EMAIL, "User@Example.COM", "Abc123456"));

        assertEquals("uid-100", response.uid());
        assertEquals("access-token", response.accessToken());
        assertEquals("refresh-token", response.refreshToken());
        assertEquals(1800, response.accessTokenExpiresIn());
        assertEquals(604800, response.refreshTokenExpiresIn());
    }

    @Test
    void loginRejectsInvalidPasswordWithoutCreatingSession() {
        User user = enabledUser("uid-100", "user@example.com");
        when(userRepository.findByContact("user@example.com")).thenReturn(user);
        when(passwordEncoder.matches("wrong", "encoded-password")).thenReturn(false);

        BusinessException exception = assertThrows(BusinessException.class,
                () -> authService.login(new LoginRequest(
                        VerificationType.EMAIL, "user@example.com", "wrong")));

        assertEquals("用户名或密码错误", exception.getMessage());
        verify(jwtTokenService, never()).generateAccessToken(anyString(), anyString(), any(Long.class));
        verify(sessionService, never()).create(anyString(), anyString());
    }

    @Test
    void registerByPhoneWritesUidAndOnlyPhone() {
        when(userRepository.findByContact("13800138000")).thenReturn(null);
        when(passwordEncoder.encode("Abc123456")).thenReturn("encoded-password");

        authService.registerByContact(VerificationType.PHONE, " 13800138000 ",
                "123456", "Abc123456");

        verify(verificationService).verify(any(), any(), anyString(), anyString());
        ArgumentCaptor<User> userCaptor = ArgumentCaptor.forClass(User.class);
        verify(userRepository).insert(userCaptor.capture());
        User savedUser = userCaptor.getValue();
        assertNotNull(savedUser.getUid());
        assertEquals(32, savedUser.getUid().length());
        assertEquals("13800138000", savedUser.getPhone());
        assertEquals(null, savedUser.getEmail());
        assertEquals("encoded-password", savedUser.getPasswordHash());
    }

    @Test
    void refreshReturnsRotatedRefreshToken() {
        when(sessionService.uidFrom("old-refresh-token")).thenReturn("uid-100");
        when(sessionService.rotate(eq("old-refresh-token"), any(String.class)))
                .thenReturn(new SessionService.Session("new-refresh-token", 604800));
        when(jwtTokenService.generateAccessToken(eq("uid-100"), any(String.class), eq(1800L)))
            .thenReturn("new-access-token");

        LoginResponse response = authService.refresh(new RefreshTokenRequest("old-refresh-token"));

        assertEquals("uid-100", response.uid());
        assertEquals("new-access-token", response.accessToken());
        assertEquals("new-refresh-token", response.refreshToken());
        verify(sessionService).rotate(eq("old-refresh-token"), any(String.class));
    }

    @Test
    void logoutBlacklistsCurrentAccessToken() {
        authService.logout("Bearer access-token");

        verify(jwtBlacklistService).blacklist("Bearer access-token");
    }

    @Test
    void forceLogoutDeletesSessionsAndBlacklistsCurrentAccessToken() {
        authService.forceLogout("uid-100", "Bearer access-token");

        verify(sessionService).deleteByUid("uid-100");
        verify(jwtBlacklistService).blacklist("Bearer access-token");
    }

    private User enabledUser(String uid, String email) {
        User user = new User();
        user.setUid(uid);
        user.setEmail(email);
        user.setPasswordHash("encoded-password");
        user.setStatus(1);
        return user;
    }
}
