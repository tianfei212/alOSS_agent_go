package oss

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

const (
	TagKeyRetentionYears = "retention-years"
	TagKeyRetentionDays  = "retention-days"
)

// RetentionUnit 保存周期单位。
type RetentionUnit string

const (
	RetentionUnitYears RetentionUnit = "years"
	RetentionUnitDays  RetentionUnit = "days"
)

// RetentionPolicy 描述对象保存周期策略。
type RetentionPolicy struct {
	Unit  RetentionUnit
	Value int
}

// RetentionYears 构造按年保存策略。
func RetentionYears(years int) RetentionPolicy {
	return RetentionPolicy{Unit: RetentionUnitYears, Value: years}
}

// RetentionDays 构造按天保存策略（OSS Lifecycle 最短 1 天）。
func RetentionDays(days int) RetentionPolicy {
	return RetentionPolicy{Unit: RetentionUnitDays, Value: days}
}

// ExpirationDays 返回 Lifecycle Expiration 天数。
func (p RetentionPolicy) ExpirationDays() int {
	if p.Unit == RetentionUnitDays {
		return p.Value
	}
	return DaysForYears(p.Value)
}

// Until 计算预计到期时间（展示用）。
func (p RetentionPolicy) Until(from time.Time) time.Time {
	if p.Unit == RetentionUnitDays {
		return from.AddDate(0, 0, p.Value)
	}
	return CalcRetentionUntil(from, p.Value)
}

// String 返回人类可读描述。
func (p RetentionPolicy) String() string {
	if p.Unit == RetentionUnitDays {
		return fmt.Sprintf("%d天", p.Value)
	}
	return fmt.Sprintf("%d年", p.Value)
}

// DaysForYears 将年数转为 Lifecycle Expiration 天数。
func DaysForYears(years int) int {
	return years * 365
}

// CalcRetentionUntil 按日历加年计算预计到期时间（展示用）。
func CalcRetentionUntil(from time.Time, years int) time.Time {
	return from.AddDate(years, 0, 0)
}

// BuildRetentionTagging 构建保存周期对象标签（years）。
func BuildRetentionTagging(years int) oss.Tagging {
	return RetentionYears(years).Tagging()
}

// Tagging 构建 OSS 对象标签。
func (p RetentionPolicy) Tagging() oss.Tagging {
	if p.Unit == RetentionUnitDays {
		return oss.Tagging{
			Tags: []oss.Tag{{
				Key:   TagKeyRetentionDays,
				Value: strconv.Itoa(p.Value),
			}},
		}
	}
	return oss.Tagging{
		Tags: []oss.Tag{{
			Key:   TagKeyRetentionYears,
			Value: strconv.Itoa(p.Value),
		}},
	}
}

// ParseRetentionPolicy 从对象标签解析保存策略。
func ParseRetentionPolicy(tags []oss.Tag) (RetentionPolicy, bool) {
	for _, t := range tags {
		if t.Key == TagKeyRetentionDays {
			d, err := strconv.Atoi(t.Value)
			if err != nil || d <= 0 {
				return RetentionPolicy{}, false
			}
			return RetentionDays(d), true
		}
	}
	for _, t := range tags {
		if t.Key == TagKeyRetentionYears {
			y, err := strconv.Atoi(t.Value)
			if err != nil || y <= 0 {
				return RetentionPolicy{}, false
			}
			return RetentionYears(y), true
		}
	}
	return RetentionPolicy{}, false
}

// RetentionFromTags 从对象标签解析保存年数；不存在时 ok=false。
func RetentionFromTags(tags []oss.Tag) (years int, ok bool) {
	p, ok := ParseRetentionPolicy(tags)
	if !ok || p.Unit != RetentionUnitYears {
		return 0, false
	}
	return p.Value, true
}

// ValidateRetentionYears 校验上传保存周期范围。
func ValidateRetentionYears(years int, maxYears int) error {
	if years <= 0 {
		return fmt.Errorf("retention_years 必须为正整数")
	}
	if maxYears <= 0 {
		maxYears = 50
	}
	if years > maxYears {
		return fmt.Errorf("retention_years 不能超过 %d", maxYears)
	}
	return nil
}

// ValidateRetentionDays 校验按天保存周期（OSS 最短 1 天）。
func ValidateRetentionDays(days int, maxDays int) error {
	if days <= 0 {
		return fmt.Errorf("retention_days 必须为正整数")
	}
	if maxDays <= 0 {
		maxDays = 365
	}
	if days > maxDays {
		return fmt.Errorf("retention_days 不能超过 %d", maxDays)
	}
	return nil
}
