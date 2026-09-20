package com.example.userservice.repository;

import java.sql.CallableStatement;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;

import org.apache.ibatis.type.BaseTypeHandler;
import org.apache.ibatis.type.JdbcType;
import org.apache.ibatis.type.MappedJdbcTypes;
import org.apache.ibatis.type.MappedTypes;

import com.example.userservice.entity.UserStatus;

@MappedTypes(UserStatus.class)
@MappedJdbcTypes(JdbcType.TINYINT)
public class UserStatusTypeHandler extends BaseTypeHandler<UserStatus> {
    @Override
    public void setNonNullParameter(PreparedStatement statement, int index,
                                    UserStatus status, JdbcType jdbcType) throws SQLException {
        statement.setInt(index, status.ordinal() + 1);
    }

    @Override
    public UserStatus getNullableResult(ResultSet resultSet, String columnName) throws SQLException {
        return fromValue(resultSet.getInt(columnName));
    }

    @Override
    public UserStatus getNullableResult(ResultSet resultSet, int columnIndex) throws SQLException {
        return fromValue(resultSet.getInt(columnIndex));
    }

    @Override
    public UserStatus getNullableResult(CallableStatement statement, int columnIndex) throws SQLException {
        return fromValue(statement.getInt(columnIndex));
    }

    private UserStatus fromValue(int value) {
        return value == 0 ? null : UserStatus.values()[value - 1];
    }
}