package com.example.userservice.dto;

/** 批量统计用的中间结果：uid + 数量。 */
public class UidCount {

    private String uid;

    private Long count;

    public String getUid() { return uid; }
    public void setUid(String uid) { this.uid = uid; }

    public Long getCount() { return count; }
    public void setCount(Long count) { this.count = count; }
}
