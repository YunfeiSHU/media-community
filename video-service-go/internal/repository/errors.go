package repository

import (
	"errors"

	"gorm.io/gorm"
)

// IsNotFound 判断数据库查询是否未找到记录。
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// IsDuplicated 判断数据库操作是否违反唯一约束。
func IsDuplicated(err error) bool { return errors.Is(err, gorm.ErrDuplicatedKey) }
