package com.example.authservice.repository;

import com.example.authservice.entity.User;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Options;
import org.apache.ibatis.annotations.Select;

@Mapper
public interface UserRepository {

    @Select("SELECT id, username, password_hash AS passwordHash, status, created_at AS createdAt, updated_at AS updatedAt "
            + "FROM `user` WHERE username = #{username} LIMIT 1")
    User findByUsername(String username);

    @Insert("INSERT INTO `user` (username, password_hash, status, created_at, updated_at) "
            + "VALUES (#{username}, #{passwordHash}, #{status}, #{createdAt}, #{updatedAt})")
    @Options(useGeneratedKeys = true, keyProperty = "id")
    int insert(User user);
}
