package com.example.authservice.service;

import org.junit.jupiter.api.AfterAll;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;

import com.example.authservice.exception.BusinessException;

@Testcontainers(disabledWithoutDocker = true)
class SessionServiceRedisIntegrationTest {

    @Container
    private static final GenericContainer<?> REDIS = new GenericContainer<>(
            DockerImageName.parse("redis:7-alpine"))
            .withExposedPorts(6379);

    private static LettuceConnectionFactory connectionFactory;
    private static StringRedisTemplate redisTemplate;
    private SessionService sessionService;

    @BeforeAll
    static void connectRedis() {
        connectionFactory = new LettuceConnectionFactory(REDIS.getHost(), REDIS.getMappedPort(6379));
        connectionFactory.afterPropertiesSet();
        redisTemplate = new StringRedisTemplate(connectionFactory);
        redisTemplate.afterPropertiesSet();
    }

    @AfterAll
    static void closeRedis() {
        if (connectionFactory != null) {
            connectionFactory.destroy();
        }
    }

    @BeforeEach
    void setUp() {
        redisTemplate.getConnectionFactory().getConnection().serverCommands().flushAll();
        sessionService = new SessionService(redisTemplate);
    }

    @Test
    void rotationDeletesOldSessionAndCreatesNewSession() {
        String oldRefreshToken = sessionService.create("uid-100").refreshToken();
        String oldKey = sessionKey(oldRefreshToken);
        assertTrue(redisTemplate.hasKey(oldKey));

        String newRefreshToken = sessionService.rotate(oldRefreshToken).refreshToken();
        String newKey = sessionKey(newRefreshToken);

        assertNotEquals(oldRefreshToken, newRefreshToken);
        assertFalse(redisTemplate.hasKey(oldKey));
        assertTrue(redisTemplate.hasKey(newKey));
        assertThrows(BusinessException.class, () -> sessionService.rotate(oldRefreshToken));
    }

    private String sessionKey(String refreshToken) {
        String[] parts = refreshToken.split("\\.", 3);
        return "auth:session:" + parts[0] + ":" + parts[1];
    }
}
