package com.example.userservice.service;

import java.time.LocalDateTime;
import java.util.List;
import java.util.UUID;

import org.springframework.dao.DuplicateKeyException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import com.example.userservice.dto.UserCreateRequest;
import com.example.userservice.dto.UserUpdateRequest;
import com.example.userservice.entity.User;
import com.example.userservice.entity.UserStatus;
import com.example.userservice.exception.ConflictException;
import com.example.userservice.exception.ResourceNotFoundException;
import com.example.userservice.repository.UserRepository;

@Service
public class UserService {
    private final UserRepository userRepository;

    public UserService(UserRepository userRepository) {
        this.userRepository = userRepository;
    }

    @Transactional
    public User create(UserCreateRequest request) {
        LocalDateTime now = LocalDateTime.now();
        User user = new User();
        user.setUid(UUID.randomUUID().toString().replace("-", ""));
        user.setNickname(request.nickname());
        user.setAvatar(request.avatar());
        user.setGender(request.gender());
        user.setBio(request.bio());
        user.setStatus(UserStatus.NORMAL);
        user.setCreatedAt(now);
        user.setUpdatedAt(now);
        try {
            userRepository.insert(user);
        } catch (DuplicateKeyException exception) {
            throw new ConflictException("username already exists");
        }
        return user;
    }

    @Transactional
    public User insertForRpc(User user) {
        userRepository.insert(user);
        return user;
    }

    public User getRequired(Long userId) {
        User user = userRepository.findById(userId);
        if (user == null) {
            throw new ResourceNotFoundException("user not found: " + userId);
        }
        return user;
    }

    public User getRequiredByUid(String uid) {
        User user = userRepository.findByUid(uid);
        if (user == null) {
            throw new ResourceNotFoundException("user not found: " + uid);
        }
        return user;
    }

    public List<User> getByIds(List<Long> userIds) {
        return userIds.isEmpty() ? List.of() : userRepository.findByIds(userIds);
    }

    public List<User> getByUids(List<String> uids) {
        return uids.isEmpty() ? List.of() : userRepository.findByUids(uids);
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
    public User updateStatus(Long userId, UserStatus status) {
        User user = getRequired(userId);
        userRepository.updateStatus(userId, status, LocalDateTime.now());
        user.setStatus(status);
        user.setUpdatedAt(LocalDateTime.now());
        return user;
    }

    public boolean exists(Long userId) {
        return userRepository.existsById(userId);
    }
}
