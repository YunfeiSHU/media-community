package com.example.userservice;

import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.web.servlet.MockMvc;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.example.userservice.entity.User;
import com.example.userservice.entity.UserRole;
import com.example.userservice.entity.UserStatus;
import com.example.userservice.repository.UserRepository;
import com.example.userservice.rpc.CreateUserCommand;
import com.example.userservice.rpc.RpcUser;
import com.example.userservice.rpc.UserDubboService;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class UserControllerIntegrationTest {
    @Autowired
    private MockMvc mockMvc;

        @Autowired
        private UserRepository userRepository;

        @Autowired
        private UserDubboService userDubboService;

    @Test
    void createsAndReadsUser() throws Exception {
        User user = createUser("demo-user", "演示用户");

        mockMvc.perform(get("/api/v1/admin/users/{userId}", user.getId()).header("X-User-Role", "ADMIN"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.nickname").value("演示用户"));
    }

    @Test
    void userCanOnlyDeleteOwnAccount() throws Exception {
        User user = createUser("demo", "资料用户");

        mockMvc.perform(delete("/api/v1/users/me").header("X-User-Id", user.getUid()))
                .andExpect(status().isNoContent());

        mockMvc.perform(get("/api/v1/users/me").header("X-User-Id", user.getUid()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.status").value("DELETED"));
    }

    @Test
    void updatesOwnProfileAndReadsStatus() throws Exception {
                User user = createUser("profile-user", "原昵称");

        mockMvc.perform(org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put("/api/v1/users/me")
                                                .header("X-User-Id", user.getUid())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"nickname\":\"新昵称\",\"bio\":\"简介\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.nickname").value("新昵称"))
                .andExpect(jsonPath("$.bio").value("简介"));

        mockMvc.perform(get("/api/v1/admin/users/{userId}", user.getId()).header("X-User-Role", "ADMIN"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.status").value("NORMAL"));
    }

    @Test
    void adminCanChangeStatusButNormalUserCannot() throws Exception {
                long userId = createUser("status-user", "状态用户").getId();

        mockMvc.perform(org.springframework.test.web.servlet.request.MockMvcRequestBuilders.patch("/api/v1/admin/users/{userId}/status", userId)
                        .header("X-User-Id", "1")
                        .header("X-User-Role", "USER")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"status\":\"DISABLED\"}"))
                .andExpect(status().isForbidden());

        mockMvc.perform(org.springframework.test.web.servlet.request.MockMvcRequestBuilders.patch("/api/v1/admin/users/{userId}/status", userId)
                        .header("X-User-Id", "1")
                        .header("X-User-Role", "ADMIN")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content("{\"status\":\"DISABLED\"}"))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.status").value("DISABLED"));
    }

    @Test
    void validatesMissingUserAndDuplicateUserErrors() throws Exception {
        mockMvc.perform(get("/api/v1/users/me"))
                .andExpect(status().isBadRequest());

        mockMvc.perform(get("/api/v1/admin/users/999999").header("X-User-Role", "ADMIN"))
                .andExpect(status().isNotFound());
    }

    @Test
    void supportsDubboCrudQueries() {
        createUser("dubbo-profile-user", "资料用户");
        RpcUser created = userDubboService.createUser(
                new CreateUserCommand("dubbo-user", "Dubbo 用户", null, null, null));

        org.junit.jupiter.api.Assertions.assertNotNull(created.getUid());
        org.junit.jupiter.api.Assertions.assertEquals(1, created.getRole());
        org.junit.jupiter.api.Assertions.assertEquals(created.getUid(),
                userDubboService.getUser(created.getUid()).getUid());
        org.junit.jupiter.api.Assertions.assertEquals(2,
                userDubboService.batchGetUsers(java.util.List.of("dubbo-profile-user", created.getUid())).size());
        org.junit.jupiter.api.Assertions.assertTrue(userDubboService
                .checkUserExists(java.util.List.of(created.getUid(), "missing-uid"))
                .get(created.getUid()));
        org.junit.jupiter.api.Assertions.assertFalse(userDubboService
                .checkUserExists(java.util.List.of(created.getUid(), "missing-uid"))
                .get("missing-uid"));
    }

        private User createUser(String username, String nickname) {
        User user = new User();
        user.setUid(username);
        user.setNickname(nickname);
                user.setRole(UserRole.USER.getCode());
                user.setStatusEnum(UserStatus.NORMAL);
        user.setCreatedAt(java.time.LocalDateTime.now());
        user.setUpdatedAt(java.time.LocalDateTime.now());
        userRepository.insert(user);
        return user;
    }
}
