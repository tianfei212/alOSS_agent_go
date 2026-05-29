package dashscope

import "time"

const (
	// DefaultBaseURL 百炼 uploads API 默认基地址。
	DefaultBaseURL = "https://dashscope.aliyuncs.com"
	// UploadExpireHours 临时 oss:// URL 的有效期（小时）。
	UploadExpireHours = 48
)

// PolicyResponse 对应百炼 getPolicy 接口的完整响应体。
type PolicyResponse struct {
	RequestID string           `json:"request_id"`
	Data      UploadPolicyData `json:"data"`
}

// UploadPolicyData 对应百炼 getPolicy 响应中的 data 字段。
type UploadPolicyData struct {
	Policy              string `json:"policy"`
	Signature           string `json:"signature"`
	UploadDir           string `json:"upload_dir"`
	UploadHost          string `json:"upload_host"`
	ExpireInSeconds     int    `json:"expire_in_seconds"`
	MaxFileSizeMB       int    `json:"max_file_size_mb"`
	CapacityLimitMB     int    `json:"capacity_limit_mb"`
	OSSAccessKeyID      string `json:"oss_access_key_id"`
	XOSSObjectACL       string `json:"x_oss_object_acl"`
	XOSSForbidOverwrite string `json:"x_oss_forbid_overwrite"`
}

// UploadResult 一站式上传成功后返回给 CLI/HTTP 调用方的结果。
type UploadResult struct {
	OSSURL    string    `json:"oss_url"`
	ExpiresAt time.Time `json:"expires_at"`
	Model     string    `json:"model"`
	Filename  string    `json:"filename"`
}

// APIError 表示百炼上游返回的业务错误，携带 HTTP 状态码。
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

// Error 实现 error 接口，返回可读的错误描述。
func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Body
}

// IsPolicyExpired 判断错误是否为 policy 凭证过期（用于自动重试）。
func IsPolicyExpired(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsPolicyExpired(msg)
}

func containsPolicyExpired(msg string) bool {
	return len(msg) > 0 && (contains(msg, "Policy expired") || contains(msg, "policy expired"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
