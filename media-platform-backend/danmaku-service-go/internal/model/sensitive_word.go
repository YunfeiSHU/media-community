package model

import "time"

// SensitiveWord 对应表 media_sensitive_word。
//
// `uk_word` + `enabled` 的设计意图：停用而不是删除，
// 便于运营回滚（误加的词停用即可，历史可追溯）。
type SensitiveWord struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Word      string    `gorm:"column:word;size:64;not null;uniqueIndex:uk_word" json:"word"`
	Strategy  string    `gorm:"column:strategy;size:16;not null;default:'REJECT'" json:"strategy"`
	Enabled   bool      `gorm:"column:enabled;not null;default:true" json:"enabled"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

// TableName 固定表名。
func (SensitiveWord) TableName() string { return "media_sensitive_word" }
