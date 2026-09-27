package com.example.authservice.repository;

import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Options;
import org.apache.ibatis.annotations.Select;

import com.example.authservice.entity.Account;

/**
 * 账号表 Mapper。
 *
 * <p>关于 {@code username}：本系统只支持手机号 / 邮箱注册，没有“账号名”这个概念，
 * 因此该列在当前链路里<b>始终为 NULL</b>（INSERT 里会写进去，但实体一直是默认 null）。
 * 保留它是因为 `account`.username 上有 {@code uk_username} 唯一索引、预留给将来的“用户名登录”；
 * MySQL / H2 的唯一索引都允许多个 NULL，所以不会互相冲突。
 */
@Mapper
public interface AccountRepository {

    /**
     * 按登录凭证（手机号或邮箱）查账号。
     *
     * <p>两个参数都绑定同一个 {@code target}，调用方（{@code AuthService}）已把凭证按是否含 {@code @} 归一化，
     * 因此不会出现“手机号格式的邮箱”之类歧义匹配。
     */
    @Select("SELECT id, uid, username, phone, email, password_hash AS passwordHash, status, created_at AS createdAt, updated_at AS updatedAt "
            + "FROM account WHERE phone = #{target} OR email = #{target} LIMIT 1")
    Account findByContact(String target);

    @Insert("INSERT INTO account (uid, username, phone, email, password_hash, status, created_at, updated_at) "
            + "VALUES (#{uid}, #{username}, #{phone}, #{email}, #{passwordHash}, #{status}, #{createdAt}, #{updatedAt})")
    @Options(useGeneratedKeys = true, keyProperty = "id")
    int insert(Account user);
}
