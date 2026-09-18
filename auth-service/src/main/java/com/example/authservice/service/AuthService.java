package com.example.authservice.service;

import com.example.authservice.dto.LoginRequest;
import com.example.authservice.dto.RegisterRequest;
import com.example.authservice.entity.User;
import com.example.authservice.exception.BusinessException;
import com.example.authservice.repository.UserRepository;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDateTime;

@Service
public class AuthService {

    private static final int SUCCESS_CODE = 0;
    private static final int BUSINESS_ERROR_CODE = 1001;

    private final UserRepository userRepository;
    private final PasswordEncoder passwordEncoder;

    public AuthService(UserRepository userRepository, PasswordEncoder passwordEncoder) {
        this.userRepository = userRepository;
        this.passwordEncoder = passwordEncoder;
    }

    @Transactional
    public void register(RegisterRequest request) {
        if (userRepository.findByUsername(request.username()) != null) {
            throw new BusinessException(BUSINESS_ERROR_CODE, "username 已存在");
        }

        LocalDateTime now = LocalDateTime.now();
        User user = new User();
        user.setUsername(request.username());
        user.setPasswordHash(passwordEncoder.encode(request.password()));
        user.setStatus(1);
        user.setCreatedAt(now);
        user.setUpdatedAt(now);

        try {
            userRepository.insert(user);
        } catch (DuplicateKeyException exception) {
            throw new BusinessException(BUSINESS_ERROR_CODE, "username 已存在");
        }
    }

    public void login(LoginRequest request) {
        User user = userRepository.findByUsername(request.username());
        if (user == null || !Integer.valueOf(1).equals(user.getStatus())
                || !passwordEncoder.matches(request.password(), user.getPasswordHash())) {
            throw new BusinessException(BUSINESS_ERROR_CODE, "用户名或密码错误");
        }
    }
}
