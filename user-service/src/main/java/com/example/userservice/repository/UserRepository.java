package com.example.userservice.repository;

import java.time.LocalDateTime;
import java.util.List;

import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Options;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import com.example.userservice.entity.User;

@Mapper
public interface UserRepository {

    @Insert("""
            INSERT INTO `user` (uid, nickname, avatar, gender, bio, role, status, created_at, updated_at)
            VALUES (#{uid}, #{nickname}, #{avatar}, #{gender}, #{bio}, #{role}, #{status,jdbcType=TINYINT}, #{createdAt}, #{updatedAt})
            """)
    @Options(useGeneratedKeys = true, keyProperty = "id", keyColumn = "id")
    int insert(User user);

        @Select("SELECT * FROM `user` WHERE id = #{userId}")
    User findById(Long userId);

        @Select("SELECT * FROM `user` WHERE uid = #{uid}")
        User findByUid(String uid);

        @Select({"<script>", "SELECT * FROM `user` WHERE uid IN",
            "<foreach item='uid' collection='uids' open='(' separator=',' close=')'>#{uid}</foreach>",
            "ORDER BY id", "</script>"})
        List<User> findByUids(List<String> uids);

        @Select({"<script>", "SELECT * FROM `user` WHERE id IN",
            "<foreach item='id' collection='userIds' open='(' separator=',' close=')'>#{id}</foreach>",
            "ORDER BY id", "</script>"})
    List<User> findByIds(List<Long> userIds);

    @Update("""
            UPDATE `user` SET nickname = #{nickname}, avatar = #{avatar}, gender = #{gender},
            bio = #{bio}, updated_at = #{updatedAt} WHERE id = #{id}
            """)
    int updateProfile(User user);

        @Update("UPDATE `user` SET status = #{status,jdbcType=TINYINT}, updated_at = #{updatedAt} WHERE id = #{userId}")
    int updateStatus(Long userId, Integer status, LocalDateTime updatedAt);

        @Select("SELECT COUNT(1) > 0 FROM `user` WHERE id = #{userId}")
    boolean existsById(Long userId);
}
