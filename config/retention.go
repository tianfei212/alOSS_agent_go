package config

const (
	DefaultRetentionYearsFallback = 2
	MaxRetentionYears             = 50
	MaxRetentionDays              = 365
)

// AllowedRetentionDays 返回允许的按天保存周期（OSS 最短 1 天，用于测试）。
func AllowedRetentionDays() []int {
	if len(AppConfig.OSS.AllowedRetentionDays) > 0 {
		return append([]int(nil), AppConfig.OSS.AllowedRetentionDays...)
	}
	return []int{1}
}

// IsAllowedRetentionDays 判断天数是否在允许列表中。
func IsAllowedRetentionDays(days int) bool {
	for _, d := range AllowedRetentionDays() {
		if d == days {
			return true
		}
	}
	return false
}

// DefaultRetentionYears 返回默认保存周期（年），未配置或无效时为 2。
func DefaultRetentionYears() int {
	y := AppConfig.OSS.DefaultRetentionYears
	if y <= 0 {
		return DefaultRetentionYearsFallback
	}
	return y
}

// AllowedRetentionYears 返回允许的上传保存周期列表。
func AllowedRetentionYears() []int {
	if len(AppConfig.OSS.AllowedRetentionYears) > 0 {
		return append([]int(nil), AppConfig.OSS.AllowedRetentionYears...)
	}
	def := DefaultRetentionYears()
	return []int{def, 3, 5, 10}
}

// IsAllowedRetentionYears 判断年限是否在允许列表中。
func IsAllowedRetentionYears(years int) bool {
	for _, y := range AllowedRetentionYears() {
		if y == years {
			return true
		}
	}
	return false
}
