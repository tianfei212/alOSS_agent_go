package oss

import (
	"testing"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

func TestDaysForYears(t *testing.T) {
	if got := DaysForYears(3); got != 1095 {
		t.Fatalf("DaysForYears(3) = %d, want 1095", got)
	}
}

func TestCalcRetentionUntil(t *testing.T) {
	from := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	until := CalcRetentionUntil(from, 2)
	want := time.Date(2028, 5, 29, 12, 0, 0, 0, time.UTC)
	if !until.Equal(want) {
		t.Fatalf("CalcRetentionUntil() = %v, want %v", until, want)
	}
}

func TestValidateRetentionYears(t *testing.T) {
	if err := ValidateRetentionYears(2, 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateRetentionYears(0, 50); err == nil {
		t.Fatal("expected error for 0")
	}
	if err := ValidateRetentionYears(51, 50); err == nil {
		t.Fatal("expected error for 51")
	}
}

func TestRetentionDaysPolicy(t *testing.T) {
	p := RetentionDays(1)
	if p.ExpirationDays() != 1 {
		t.Fatalf("ExpirationDays = %d", p.ExpirationDays())
	}
	tag := p.Tagging()
	if len(tag.Tags) != 1 || tag.Tags[0].Key != TagKeyRetentionDays || tag.Tags[0].Value != "1" {
		t.Fatalf("unexpected tagging: %+v", tag.Tags)
	}
}

func TestParseRetentionPolicyDays(t *testing.T) {
	p, ok := ParseRetentionPolicy([]oss.Tag{{Key: TagKeyRetentionDays, Value: "1"}})
	if !ok || p.Unit != RetentionUnitDays || p.Value != 1 {
		t.Fatalf("got %+v ok=%v", p, ok)
	}
}

func TestMergeRetentionLifecycleRules(t *testing.T) {
	existing := []oss.LifecycleRule{
		{ID: "other-rule", Prefix: "logs/", Status: "Enabled"},
		{ID: "retention-years-2", Prefix: "a/", Status: "Enabled"},
	}
	ours := []oss.LifecycleRule{
		BuildRetentionLifecycleRule("video/", 3),
	}
	merged := MergeRetentionLifecycleRules(existing, ours)
	if len(merged) != 2 {
		t.Fatalf("len(merged) = %d, want 2", len(merged))
	}
	if merged[0].ID != "other-rule" {
		t.Fatalf("kept rule = %s", merged[0].ID)
	}
	if merged[1].ID != "retention-years-3" {
		t.Fatalf("new rule = %s", merged[1].ID)
	}
}

func TestMatchLifecycleForObject(t *testing.T) {
	rules := []oss.LifecycleRule{
		BuildRetentionLifecycleRule("video_T/", 3),
	}
	key := "video_T/foo.mp4"
	tags := []oss.Tag{{Key: TagKeyRetentionYears, Value: "3"}}
	match := MatchLifecycleForObject(key, tags, rules)
	if match.ExpireDays != 1095 {
		t.Fatalf("ExpireDays = %d", match.ExpireDays)
	}
}
