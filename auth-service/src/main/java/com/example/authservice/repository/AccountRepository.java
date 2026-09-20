package com.example.authservice.repository;

import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Options;
import org.apache.ibatis.annotations.Select;

import com.example.authservice.entity.Account;

@Mapper
public interface AccountRepository {

    @Select("SELECT id, uid, username, phone, email, password_hash AS passwordHash, status, created_at AS createdAt, updated_at AS updatedAt "
                        + "FROM account WHERE phone = #{target} OR email = #{target} LIMIT 1")
    Account findByContact(String target);

        @Insert("INSERT INTO account (uid, username, phone, email, password_hash, status, created_at, updated_at) "
            + "VALUES (#{uid}, #{username}, #{phone}, #{email}, #{passwordHash}, #{status}, #{createdAt}, #{updatedAt})")
    @Options(useGeneratedKeys = true, keyProperty = "id")
    int insert(Account user);
}
