package dashscope

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildOSSURL(t *testing.T) {
	tests := []struct {
		uploadDir string
		filename  string
		want      string
	}{
		{
			uploadDir: "dashscope-instant/xxx/2024-07-18/xxx",
			filename:  "cat.png",
			want:      "oss://dashscope-instant/xxx/2024-07-18/xxx/cat.png",
		},
		{
			uploadDir: "dashscope-instant/user/2026-05-29",
			filename:  "video.mp4",
			want:      "oss://dashscope-instant/user/2026-05-29/video.mp4",
		},
	}

	for _, tt := range tests {
		got := BuildOSSURL(tt.uploadDir, tt.filename)
		if got != tt.want {
			t.Errorf("BuildOSSURL(%q, %q) = %q, want %q", tt.uploadDir, tt.filename, got, tt.want)
		}
	}
}

func TestCalcExpiresAt(t *testing.T) {
	uploadTime := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	expires := CalcExpiresAt(uploadTime)
	expected := uploadTime.Add(48 * time.Hour)
	if !expires.Equal(expected) {
		t.Errorf("CalcExpiresAt() = %v, want %v", expires, expected)
	}
}

func TestPolicyResponseJSON(t *testing.T) {
	raw := `{
		"request_id": "52f4383a-c67d-9f8c-xxxxxx",
		"data": {
			"policy": "eyJl...1ZSJ=",
			"signature": "eWy...=",
			"upload_dir": "dashscope-instant/xxx/2024-07-18/xxx",
			"upload_host": "https://dashscope-file-xxx.oss-cn-beijing.aliyuncs.com",
			"expire_in_seconds": 300,
			"max_file_size_mb": 100,
			"capacity_limit_mb": 999999999,
			"oss_access_key_id": "LTAxxx",
			"x_oss_object_acl": "private",
			"x_oss_forbid_overwrite": "true"
		}
	}`

	var resp PolicyResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}

	if resp.RequestID != "52f4383a-c67d-9f8c-xxxxxx" {
		t.Errorf("request_id = %q", resp.RequestID)
	}
	if resp.Data.UploadDir != "dashscope-instant/xxx/2024-07-18/xxx" {
		t.Errorf("upload_dir = %q", resp.Data.UploadDir)
	}
	if resp.Data.ExpireInSeconds != 300 {
		t.Errorf("expire_in_seconds = %d", resp.Data.ExpireInSeconds)
	}
	if resp.Data.MaxFileSizeMB != 100 {
		t.Errorf("max_file_size_mb = %d", resp.Data.MaxFileSizeMB)
	}

	ossURL := BuildOSSURL(resp.Data.UploadDir, "cat.png")
	want := "oss://dashscope-instant/xxx/2024-07-18/xxx/cat.png"
	if ossURL != want {
		t.Errorf("BuildOSSURL = %q, want %q", ossURL, want)
	}
}

func TestIsPolicyExpired(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{&APIError{Message: "Invalid according to Policy: Policy expired."}, true},
		{&APIError{Message: "some other error"}, false},
	}

	for _, tt := range tests {
		got := IsPolicyExpired(tt.err)
		if got != tt.want {
			t.Errorf("IsPolicyExpired(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

func TestContainsPolicyExpired(t *testing.T) {
	if !containsPolicyExpired("Invalid according to Policy: Policy expired.") {
		t.Error("应识别 Policy expired")
	}
	if containsPolicyExpired("network timeout") {
		t.Error("不应误判为 Policy expired")
	}
}
