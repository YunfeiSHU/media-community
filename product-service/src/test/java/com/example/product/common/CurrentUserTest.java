package com.example.product.common;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.mock.web.MockHttpServletRequest;

/**
 * 当前用户上下文的行为约定：身份与角色都只能来自网关注入的请求头，
 * 请求参数 / 其它头 / 已废弃的历史头都不是可信来源。
 */
class CurrentUserTest {

    @Test
    @DisplayName("uid 只取 X-User-Id，已废弃的历史头 uid 不再被信任")
    void uidComesFromGatewayHeader() {
        MockHttpServletRequest request = new MockHttpServletRequest();
        request.addHeader("X-User-Id", "u-1001");
        request.addHeader("uid", "legacy-uid");
        assertThat(CurrentUser.uid(request)).isEqualTo("u-1001");

        // 只带历史头 → 40101（网关只会删它、不会写它）
        MockHttpServletRequest legacyOnly = new MockHttpServletRequest();
        legacyOnly.addHeader("uid", "legacy-uid");
        assertThatThrownBy(() -> CurrentUser.uid(legacyOnly))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.UNAUTHORIZED);
    }

    @Test
    @DisplayName("缺身份头（含只在参数里写 uid）→ 40101")
    void missingUidIsUnauthorized() {
        MockHttpServletRequest request = new MockHttpServletRequest();
        request.addParameter("uid", "forged-uid");

        assertThatThrownBy(() -> CurrentUser.uid(request))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.UNAUTHORIZED);
    }

    @Test
    @DisplayName("uid 超长 → 40001（与 product.uid VARCHAR(32) 对齐）")
    void tooLongUidIsInvalidParam() {
        assertThatThrownBy(() -> CurrentUser.requireUid("u".repeat(33)))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.INVALID_PARAM);
    }

    @Test
    @DisplayName("角色只取 X-User-Role，参数/其它头里的角色一律无效")
    void roleComesOnlyFromGatewayHeader() {
        MockHttpServletRequest request = new MockHttpServletRequest();
        request.addHeader("X-User-Id", "u-1001");
        request.addParameter("role", "ADMIN");
        request.addHeader("Role", "ADMIN");

        assertThat(CurrentUser.role(request)).isNull();
        assertThatThrownBy(() -> CurrentUser.requireAdmin(request))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.FORBIDDEN);
    }

    @Test
    @DisplayName("角色来自 X-User-Role 时按忽略大小写匹配")
    void roleMatchesIgnoringCase() {
        MockHttpServletRequest request = new MockHttpServletRequest();
        request.addHeader("X-User-Id", "u-1001");
        request.addHeader("X-User-Role", "admin");

        assertThat(CurrentUser.role(request)).isEqualTo("admin");
        assertThat(CurrentUser.role(request)).isEqualToIgnoringCase("ADMIN");
        CurrentUser.requireAdmin(request);
        CurrentUser.requireRole(request, "ADMIN");
    }

    @Test
    @DisplayName("角色不匹配 → 40301；缺身份头 → 40101（未认证优先于鉴权）")
    void requireRoleFailures() {
        MockHttpServletRequest normalUser = new MockHttpServletRequest();
        normalUser.addHeader("X-User-Id", "u-1001");
        normalUser.addHeader("X-User-Role", "user");
        assertThatThrownBy(() -> CurrentUser.requireRole(normalUser, "ADMIN"))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.FORBIDDEN);

        MockHttpServletRequest noIdentity = new MockHttpServletRequest();
        noIdentity.addHeader("X-User-Role", "ADMIN");
        assertThatThrownBy(() -> CurrentUser.requireAdmin(noIdentity))
                .isInstanceOf(BizException.class)
                .extracting(exception -> ((BizException) exception).getErrorCode())
                .isEqualTo(ErrorCode.UNAUTHORIZED);
    }
}
