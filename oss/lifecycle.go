package oss

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

const (
	retentionRuleYearsPrefix = "retention-years-"
	retentionRuleDaysPrefix  = "retention-days-"
)

func isManagedRetentionRuleID(id string) bool {
	return strings.HasPrefix(id, retentionRuleYearsPrefix) || strings.HasPrefix(id, retentionRuleDaysPrefix)
}

// BuildRetentionLifecycleRule 构建带 Tag 筛选的 Expiration 生命周期规则（按年）。
func BuildRetentionLifecycleRule(prefix string, years int) oss.LifecycleRule {
	return buildRetentionLifecycleRule(prefix, retentionRuleYearsPrefix, TagKeyRetentionYears, years, DaysForYears(years))
}

// BuildRetentionDaysLifecycleRule 构建按天的 Lifecycle 规则（测试/短周期）。
func BuildRetentionDaysLifecycleRule(prefix string, days int) oss.LifecycleRule {
	return buildRetentionLifecycleRule(prefix, retentionRuleDaysPrefix, TagKeyRetentionDays, days, days)
}

func buildRetentionLifecycleRule(prefix, idPrefix, tagKey string, tagValue, expireDays int) oss.LifecycleRule {
	p := prefix
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return oss.LifecycleRule{
		ID:     idPrefix + strconv.Itoa(tagValue),
		Prefix: p,
		Status: "Enabled",
		Tags: []oss.Tag{{
			Key:   tagKey,
			Value: strconv.Itoa(tagValue),
		}},
		Expiration: &oss.LifecycleExpiration{
			Days: expireDays,
		},
	}
}

// MergeRetentionLifecycleRules 合并 retention 规则：保留非本系统规则，upsert ours。
func MergeRetentionLifecycleRules(existing []oss.LifecycleRule, ours []oss.LifecycleRule) []oss.LifecycleRule {
	kept := make([]oss.LifecycleRule, 0, len(existing))
	for _, r := range existing {
		if !isManagedRetentionRuleID(r.ID) {
			kept = append(kept, r)
		}
	}
	return append(kept, ours...)
}

// HasRetentionLifecycleRule 检查是否存在指定年数的 retention 生命周期规则。
func HasRetentionLifecycleRule(rules []oss.LifecycleRule, years int) bool {
	wantID := retentionRuleYearsPrefix + strconv.Itoa(years)
	return hasLifecycleRuleID(rules, wantID)
}

// HasRetentionDaysLifecycleRule 检查是否存在指定天数的 retention 生命周期规则。
func HasRetentionDaysLifecycleRule(rules []oss.LifecycleRule, days int) bool {
	wantID := retentionRuleDaysPrefix + strconv.Itoa(days)
	return hasLifecycleRuleID(rules, wantID)
}

func hasLifecycleRuleID(rules []oss.LifecycleRule, wantID string) bool {
	for _, r := range rules {
		if r.ID != wantID {
			continue
		}
		if strings.ToLower(r.Status) != "enabled" {
			continue
		}
		if r.Expiration == nil || r.Expiration.Days <= 0 {
			continue
		}
		return true
	}
	return false
}

// LifecycleMatch 描述对象命中的生命周期规则。
type LifecycleMatch struct {
	RuleID     string
	Label      string
	ExpireDays int
}

// MatchLifecycleForObject 按 prefix + 对象 tag 匹配 Lifecycle 规则。
func MatchLifecycleForObject(objectKey string, objectTags []oss.Tag, rules []oss.LifecycleRule) LifecycleMatch {
	var best LifecycleMatch
	bestPrefixLen := -1

	policy, hasPolicy := ParseRetentionPolicy(objectTags)

	for _, rule := range rules {
		if strings.ToLower(rule.Status) != "enabled" {
			continue
		}
		if rule.Prefix != "" && !strings.HasPrefix(objectKey, rule.Prefix) {
			continue
		}
		if len(rule.Tags) > 0 {
			if !hasPolicy {
				continue
			}
			matched := false
			for _, rt := range rule.Tags {
				tagPolicy, tagOK := parseTagPair(rt.Key, rt.Value)
				if tagOK && tagPolicy.Unit == policy.Unit && tagPolicy.Value == policy.Value {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		prefixLen := len(rule.Prefix)
		if prefixLen <= bestPrefixLen {
			continue
		}
		info := LifecycleMatch{RuleID: rule.ID, Label: "无自动删除规则"}
		if rule.Expiration != nil && rule.Expiration.Days > 0 {
			info.ExpireDays = rule.Expiration.Days
			if hasPolicy {
				info.Label = fmt.Sprintf("%d 天（自最后修改日起，tag=%s）", rule.Expiration.Days, policy.String())
			} else {
				info.Label = fmt.Sprintf("%d 天（自最后修改日起）", rule.Expiration.Days)
			}
		} else if rule.ID != "" {
			info.Label = "规则 " + rule.ID
		}
		best = info
		bestPrefixLen = prefixLen
	}

	if bestPrefixLen < 0 {
		if hasPolicy {
			return LifecycleMatch{Label: fmt.Sprintf("tag=%s，无匹配 Lifecycle 规则", policy.String())}
		}
		return LifecycleMatch{Label: "无匹配规则"}
	}
	return best
}

func parseTagPair(key, value string) (RetentionPolicy, bool) {
	v, err := strconv.Atoi(value)
	if err != nil || v <= 0 {
		return RetentionPolicy{}, false
	}
	switch key {
	case TagKeyRetentionDays:
		return RetentionDays(v), true
	case TagKeyRetentionYears:
		return RetentionYears(v), true
	default:
		return RetentionPolicy{}, false
	}
}

func isNoLifecycleConfig(err error) bool {
	var se oss.ServiceError
	if errors.As(err, &se) {
		return se.Code == "NoSuchLifecycle" || se.Code == "NoSuchLifecycleConfigurationOnBucket"
	}
	return false
}

// GetBucketLifecycleRules 获取 Bucket 生命周期规则。
func (c *Client) GetBucketLifecycleRules() ([]oss.LifecycleRule, error) {
	res, err := c.Bucket.Client.GetBucketLifecycle(c.Bucket.BucketName)
	if err != nil {
		if isNoLifecycleConfig(err) {
			return nil, nil
		}
		return nil, err
	}
	return res.Rules, nil
}

// SetBucketLifecycleRules 全量设置 Bucket 生命周期规则。
func (c *Client) SetBucketLifecycleRules(rules []oss.LifecycleRule) error {
	return c.Bucket.Client.SetBucketLifecycle(c.Bucket.BucketName, rules)
}

// SyncRetentionLifecycleRules 同步按年/按天的 retention Lifecycle 规则。
func (c *Client) SyncRetentionLifecycleRules(prefix string, yearsList, daysList []int) error {
	existing, err := c.GetBucketLifecycleRules()
	if err != nil {
		return fmt.Errorf("获取生命周期规则失败: %w", err)
	}
	ours := make([]oss.LifecycleRule, 0, len(yearsList)+len(daysList))
	for _, y := range yearsList {
		ours = append(ours, BuildRetentionLifecycleRule(prefix, y))
	}
	for _, d := range daysList {
		ours = append(ours, BuildRetentionDaysLifecycleRule(prefix, d))
	}
	merged := MergeRetentionLifecycleRules(existing, ours)
	return c.SetBucketLifecycleRules(merged)
}

// PutObjectRetentionTag 为对象设置 retention-years 标签。
func (c *Client) PutObjectRetentionTag(objectKey string, years int) error {
	return c.PutObjectRetentionPolicy(objectKey, RetentionYears(years))
}

// PutObjectRetentionPolicy 为对象设置保存周期标签。
func (c *Client) PutObjectRetentionPolicy(objectKey string, policy RetentionPolicy) error {
	finalKey := c.resolveKey(objectKey)
	return c.Bucket.PutObjectTagging(finalKey, policy.Tagging())
}

// GetObjectRetentionPolicy 读取对象保存周期标签。
func (c *Client) GetObjectRetentionPolicy(objectKey string) (RetentionPolicy, bool, error) {
	finalKey := c.resolveKey(objectKey)
	res, err := c.Bucket.GetObjectTagging(finalKey)
	if err != nil {
		return RetentionPolicy{}, false, fmt.Errorf("获取对象标签失败: %w", err)
	}
	p, ok := ParseRetentionPolicy(res.Tags)
	return p, ok, nil
}

// GetObjectRetentionTag 读取对象 retention-years 标签。
func (c *Client) GetObjectRetentionTag(objectKey string) (years int, ok bool, err error) {
	p, ok, err := c.GetObjectRetentionPolicy(objectKey)
	if err != nil || !ok || p.Unit != RetentionUnitYears {
		return 0, ok && p.Unit == RetentionUnitYears, err
	}
	return p.Value, true, nil
}
