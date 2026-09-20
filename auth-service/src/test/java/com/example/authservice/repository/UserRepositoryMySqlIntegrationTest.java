package com.example.authservice.repository;

import java.time.LocalDateTime;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

import javax.sql.DataSource;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mybatis.spring.boot.test.autoconfigure.MybatisTest;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.jdbc.AutoConfigureTestDatabase;
import static org.springframework.boot.test.autoconfigure.jdbc.AutoConfigureTestDatabase.Replace.NONE;
import org.springframework.core.io.ClassPathResource;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.jdbc.datasource.init.ResourceDatabasePopulator;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.MySQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;

import com.example.authservice.entity.Account;

@Testcontainers(disabledWithoutDocker = true)
@MybatisTest
@AutoConfigureTestDatabase(replace = NONE)
class UserRepositoryMySqlIntegrationTest {

    @Container
    private static final MySQLContainer<?> MYSQL = new MySQLContainer<>(
            DockerImageName.parse("mysql:8.4"))
            .withDatabaseName("auth_service")
            .withUsername("test")
            .withPassword("test");

    @DynamicPropertySource
    static void configureDataSource(DynamicPropertyRegistry registry) {
        registry.add("spring.datasource.url", MYSQL::getJdbcUrl);
        registry.add("spring.datasource.username", MYSQL::getUsername);
        registry.add("spring.datasource.password", MYSQL::getPassword);
        registry.add("spring.datasource.driver-class-name", MYSQL::getDriverClassName);
        registry.add("spring.sql.init.mode", () -> "never");
    }

    @Autowired
    private AccountRepository userRepository;

    @Autowired
    private DataSource dataSource;

    @BeforeEach
    void setUp() {
        new ResourceDatabasePopulator(new ClassPathResource("schema.sql"))
                .execute(dataSource);
    }

    @Test
    void insertsAndReadsPhoneUserFromRealMySql() {
        Account user = user("uid-phone-001", "13800138000", null, 1);

        assertEquals(1, userRepository.insert(user));
        Account loaded = userRepository.findByContact("13800138000");

        assertNotNull(loaded);
        assertEquals("uid-phone-001", loaded.getUid());
        assertEquals("13800138000", loaded.getPhone());
        assertNull(loaded.getEmail());
        assertEquals(1, loaded.getStatus());
    }

    @Test
    void uniqueIndexesRejectDuplicateUidPhoneAndEmail() {
        userRepository.insert(user("uid-unique-001", "13800138001", "one@example.com", 1));

        assertThrows(DuplicateKeyException.class,
                () -> userRepository.insert(user("uid-unique-001", "13800138002", null, 1)));
        assertThrows(DuplicateKeyException.class,
                () -> userRepository.insert(user("uid-unique-002", "13800138001", null, 1)));
        assertThrows(DuplicateKeyException.class,
                () -> userRepository.insert(user("uid-unique-003", null, "one@example.com", 1)));
    }

    @Test
    void readsDisabledUserButApplicationCanRejectItByStatus() {
        userRepository.insert(user("uid-disabled-001", "13800138003", null, 0));

        Account loaded = userRepository.findByContact("13800138003");

        assertNotNull(loaded);
        assertEquals(0, loaded.getStatus());
    }

    @Test
    void concurrentSamePhoneInsertAllowsOnlyOneWinner() throws Exception {
        ExecutorService executor = Executors.newFixedThreadPool(2);
        CountDownLatch ready = new CountDownLatch(2);
        CountDownLatch start = new CountDownLatch(1);
        try {
            List<Future<Boolean>> results = List.of(
                    submitInsert(executor, ready, start, "uid-race-001"),
                    submitInsert(executor, ready, start, "uid-race-002"));
            ready.await();
            start.countDown();

            long successCount = results.stream().filter(this::getResult).count();

            assertEquals(1, successCount);
            assertEquals("13800138004", userRepository.findByContact("13800138004").getPhone());
        } finally {
            executor.shutdownNow();
        }
    }

    private Future<Boolean> submitInsert(ExecutorService executor, CountDownLatch ready,
                                          CountDownLatch start, String uid) {
        return executor.submit(() -> {
            ready.countDown();
            start.await();
            try {
                userRepository.insert(user(uid, "13800138004", null, 1));
                return true;
            } catch (DuplicateKeyException exception) {
                return false;
            }
        });
    }

    private boolean getResult(Future<Boolean> result) {
        try {
            return result.get();
        } catch (Exception exception) {
            throw new AssertionError("并发写入测试执行失败", exception);
        }
    }

    private Account user(String uid, String phone, String email, int status) {
        Account user = new Account();
        user.setUid(uid);
        user.setPhone(phone);
        user.setEmail(email);
        user.setPasswordHash("encoded-password");
        user.setStatus(status);
        user.setCreatedAt(LocalDateTime.now());
        user.setUpdatedAt(LocalDateTime.now());
        return user;
    }
}
