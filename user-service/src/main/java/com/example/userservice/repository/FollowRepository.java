package com.example.userservice.repository;

import java.time.LocalDateTime;
import java.util.List;

import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import com.example.userservice.dto.FollowRow;
import com.example.userservice.dto.UidCount;

/**
 * 关注关系 Mapper（注解式 SQL）。
 *
 * <p>计数直接 COUNT(*)，不维护冗余计数列：demo 规模下足够，
 * 也避免「关系表与计数列不一致」这类只有上线后才会暴露的问题。
 */
@Mapper
public interface FollowRepository {

    /** 关注（幂等：唯一键冲突时忽略，返回 0 表示已关注）。 */
    @Insert("INSERT IGNORE INTO user_follow (follower_uid, followee_uid, created_at) "
            + "VALUES (#{followerUid}, #{followeeUid}, #{createdAt})")
    int insert(@Param("followerUid") String followerUid,
               @Param("followeeUid") String followeeUid,
               @Param("createdAt") LocalDateTime createdAt);

    @Delete("DELETE FROM user_follow WHERE follower_uid = #{followerUid} AND followee_uid = #{followeeUid}")
    int delete(@Param("followerUid") String followerUid,
               @Param("followeeUid") String followeeUid);

    @Select("SELECT COUNT(*) FROM user_follow WHERE followee_uid = #{uid}")
    long countFollowers(@Param("uid") String uid);

    @Select("SELECT COUNT(*) FROM user_follow WHERE follower_uid = #{uid}")
    long countFollowing(@Param("uid") String uid);

    /** 是否已关注（用于播放页/空间页初始化按钮状态）。 */
    @Select("SELECT COUNT(*) FROM user_follow WHERE follower_uid = #{followerUid} AND followee_uid = #{followeeUid}")
    long countRelation(@Param("followerUid") String followerUid,
                       @Param("followeeUid") String followeeUid);

    @Select({"<script>",
            "SELECT followee_uid AS uid, COUNT(*) AS count FROM user_follow",
            "WHERE followee_uid IN",
            "<foreach item='uid' collection='uids' open='(' separator=',' close=')'>#{uid}</foreach>",
            "GROUP BY followee_uid", "</script>"})
    List<UidCount> countFollowersByUids(@Param("uids") List<String> uids);

    @Select({"<script>",
            "SELECT follower_uid AS uid, COUNT(*) AS count FROM user_follow",
            "WHERE follower_uid IN",
            "<foreach item='uid' collection='uids' open='(' separator=',' close=')'>#{uid}</foreach>",
            "GROUP BY follower_uid", "</script>"})
    List<UidCount> countFollowingByUids(@Param("uids") List<String> uids);

    /** 粉丝列表：关注我的人（只返回状态正常的用户）。 */
    @Select("""
            SELECT u.uid, u.nickname, u.avatar, u.bio, u.gender, f.created_at
            FROM user_follow f
            JOIN `user` u ON u.uid = f.follower_uid
            WHERE f.followee_uid = #{uid} AND u.status = 1
            ORDER BY f.created_at DESC
            LIMIT #{limit} OFFSET #{offset}
            """)
    List<FollowRow> pageFollowers(@Param("uid") String uid,
                                  @Param("offset") int offset,
                                  @Param("limit") int limit);

    /** 关注列表：我关注的人。 */
    @Select("""
            SELECT u.uid, u.nickname, u.avatar, u.bio, u.gender, f.created_at
            FROM user_follow f
            JOIN `user` u ON u.uid = f.followee_uid
            WHERE f.follower_uid = #{uid} AND u.status = 1
            ORDER BY f.created_at DESC
            LIMIT #{limit} OFFSET #{offset}
            """)
    List<FollowRow> pageFollowing(@Param("uid") String uid,
                                  @Param("offset") int offset,
                                  @Param("limit") int limit);
}
