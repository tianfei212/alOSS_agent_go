package config

import "testing"

func TestDefaultRetentionYears(t *testing.T) {
	AppConfig.OSS.DefaultRetentionYears = 0
	if got := DefaultRetentionYears(); got != DefaultRetentionYearsFallback {
		t.Fatalf("got %d", got)
	}
	AppConfig.OSS.DefaultRetentionYears = 5
	if got := DefaultRetentionYears(); got != 5 {
		t.Fatalf("got %d", got)
	}
}

func TestAllowedRetentionYearsDefault(t *testing.T) {
	AppConfig.OSS.AllowedRetentionYears = nil
	AppConfig.OSS.DefaultRetentionYears = 2
	allowed := AllowedRetentionYears()
	if len(allowed) < 2 {
		t.Fatalf("allowed too short: %v", allowed)
	}
}

func TestIsAllowedRetentionYears(t *testing.T) {
	AppConfig.OSS.AllowedRetentionYears = []int{2, 3, 5}
	if !IsAllowedRetentionYears(3) {
		t.Fatal("3 should be allowed")
	}
	if IsAllowedRetentionYears(7) {
		t.Fatal("7 should not be allowed")
	}
}
