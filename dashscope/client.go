package dashscope

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"time"
)

// FileOpener 用于打开待上传文件；支持 policy 过期重试时重新打开文件流。
type FileOpener func() (io.ReadCloser, int64, error)

// Client 百炼临时文件上传客户端，仅依赖 AL_KEY，不依赖 oss 包。
type Client struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewClient 创建百炼临时上传客户端。
// apiKey 为 .env.local 中的 AL_KEY；baseURL 为空时使用 DefaultBaseURL。
func NewClient(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	logDebug("创建 Client，baseURL=%s，apiKey 已配置=%v", baseURL, apiKey != "")
	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    &http.Client{Timeout: 10 * time.Minute},
	}
}

// BuildOSSURL 根据 upload_dir 与文件名拼接 oss:// 临时 URL。
func BuildOSSURL(uploadDir, filename string) string {
	key := uploadDir + "/" + filename
	return "oss://" + key
}

// CalcExpiresAt 计算上传完成后的过期时间（上传时刻 + 48 小时）。
func CalcExpiresAt(uploadTime time.Time) time.Time {
	return uploadTime.Add(UploadExpireHours * time.Hour)
}

// GetUploadPolicy 调用百炼 getPolicy 接口，获取临时 OSS 上传凭证。
func (c *Client) GetUploadPolicy(ctx context.Context, model string) (*PolicyResponse, error) {
	if model == "" {
		return nil, fmt.Errorf("model 参数不能为空")
	}

	url := fmt.Sprintf("%s/api/v1/uploads?action=getPolicy&model=%s", c.baseURL, model)
	logInfo("开始请求 getPolicy，model=%s", model)
	logDebug("getPolicy 请求 URL: %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logError("创建 getPolicy 请求失败: %v", err)
		return nil, fmt.Errorf("创建 getPolicy 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		logError("getPolicy 网络请求失败: %v", err)
		return nil, fmt.Errorf("getPolicy 请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logError("读取 getPolicy 响应体失败: %v", err)
		return nil, fmt.Errorf("读取 getPolicy 响应失败: %w", err)
	}

	logDebug("getPolicy 响应状态码: %d，body 长度: %d", resp.StatusCode, len(body))

	if resp.StatusCode != http.StatusOK {
		logError("getPolicy 上游返回错误，status=%d，body=%s", resp.StatusCode, truncateBody(body))
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("getPolicy 失败 (HTTP %d)", resp.StatusCode),
			Body:       string(body),
		}
	}

	var policyResp PolicyResponse
	if err := json.Unmarshal(body, &policyResp); err != nil {
		logError("解析 getPolicy JSON 失败: %v", err)
		return nil, fmt.Errorf("解析 getPolicy 响应失败: %w", err)
	}

	logInfo("getPolicy 成功，request_id=%s，upload_dir=%s，expire_in_seconds=%d",
		policyResp.RequestID, policyResp.Data.UploadDir, policyResp.Data.ExpireInSeconds)
	return &policyResp, nil
}

// UploadWithPolicy 使用 getPolicy 凭证，以 multipart/form-data POST 到临时 upload_host。
// file 字段必须为最后一项，字段顺序严格遵循百炼官方文档。
func (c *Client) UploadWithPolicy(ctx context.Context, policy *UploadPolicyData, filename string, reader io.Reader, size int64) (string, error) {
	if policy == nil {
		return "", fmt.Errorf("upload policy 不能为空")
	}
	if filename == "" {
		return "", fmt.Errorf("filename 不能为空")
	}

	objectKey := policy.UploadDir + "/" + filename
	logInfo("开始上传文件至临时 OSS，filename=%s，upload_host=%s", filename, policy.UploadHost)
	logDebug("上传 objectKey=%s，文件大小=%d 字节", objectKey, size)

	if size > 0 && policy.MaxFileSizeMB > 0 {
		maxBytes := int64(policy.MaxFileSizeMB) * 1024 * 1024
		if size > maxBytes {
			logError("文件大小 %d 字节超出模型限制 %d MB", size, policy.MaxFileSizeMB)
			return "", fmt.Errorf("文件大小超出限制：最大 %d MB", policy.MaxFileSizeMB)
		}
	}

	body, contentType, err := buildMultipartBody(policy, filename, reader)
	if err != nil {
		logError("构建 multipart 请求体失败: %v", err)
		return "", fmt.Errorf("构建上传请求体失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, policy.UploadHost, body)
	if err != nil {
		logError("创建上传请求失败: %v", err)
		return "", fmt.Errorf("创建上传请求失败: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.http.Do(req)
	if err != nil {
		logError("上传至 upload_host 网络失败: %v", err)
		return "", fmt.Errorf("上传至临时 OSS 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logError("读取上传响应体失败: %v", err)
		return "", fmt.Errorf("读取上传响应失败: %w", err)
	}

	logDebug("上传响应状态码: %d，body=%s", resp.StatusCode, truncateBody(respBody))

	if resp.StatusCode != http.StatusOK {
		logError("上传至临时 OSS 失败，status=%d，body=%s", resp.StatusCode, truncateBody(respBody))
		return "", &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
			Body:       string(respBody),
		}
	}

	ossURL := BuildOSSURL(policy.UploadDir, filename)
	logInfo("文件上传成功，oss_url=%s", ossURL)
	return ossURL, nil
}

// UploadAndGetURL 一站式上传：getPolicy → POST 临时 OSS → 返回 oss_url 与 expires_at。
// policy 过期时自动重新 getPolicy 并重传，最多重试 1 次。
func (c *Client) UploadAndGetURL(ctx context.Context, model, filename string, open FileOpener) (*UploadResult, error) {
	if model == "" {
		return nil, fmt.Errorf("model 参数不能为空")
	}
	if open == nil {
		return nil, fmt.Errorf("文件打开函数不能为空")
	}

	ossURL, uploadedAt, err := c.uploadOnce(ctx, model, filename, open)
	if err != nil && IsPolicyExpired(err) {
		logInfo("检测到 policy 过期，正在重新获取凭证并重试上传（最多 1 次）")
		ossURL, uploadedAt, err = c.uploadOnce(ctx, model, filename, open)
	}
	if err != nil {
		return nil, err
	}

	return &UploadResult{
		OSSURL:    ossURL,
		ExpiresAt: CalcExpiresAt(uploadedAt),
		Model:     model,
		Filename:  path.Base(filename),
	}, nil
}

// uploadOnce 执行一次 getPolicy + 上传，返回 oss_url 与上传完成时间。
func (c *Client) uploadOnce(ctx context.Context, model, filename string, open FileOpener) (string, time.Time, error) {
	policyResp, err := c.GetUploadPolicy(ctx, model)
	if err != nil {
		return "", time.Time{}, err
	}

	rc, size, err := open()
	if err != nil {
		logError("打开待上传文件失败: %v", err)
		return "", time.Time{}, fmt.Errorf("打开文件失败: %w", err)
	}
	defer rc.Close()

	ossURL, err := c.UploadWithPolicy(ctx, &policyResp.Data, path.Base(filename), rc, size)
	if err != nil {
		return "", time.Time{}, err
	}
	return ossURL, time.Now(), nil
}

// buildMultipartBody 按百炼官方字段顺序构建 multipart 请求体，file 为最后一项。
func buildMultipartBody(policy *UploadPolicyData, filename string, reader io.Reader) (io.Reader, string, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		var err error
		defer func() {
			writer.Close()
			if err != nil {
				_ = pw.CloseWithError(err)
			} else {
				_ = pw.Close()
			}
		}()

		objectKey := policy.UploadDir + "/" + filename

		fields := []struct {
			name  string
			value string
		}{
			{"OSSAccessKeyId", policy.OSSAccessKeyID},
			{"Signature", policy.Signature},
			{"policy", policy.Policy},
			{"x-oss-object-acl", policy.XOSSObjectACL},
			{"x-oss-forbid-overwrite", policy.XOSSForbidOverwrite},
			{"key", objectKey},
			{"success_action_status", "200"},
		}

		for _, f := range fields {
			if err = writer.WriteField(f.name, f.value); err != nil {
				logError("写入 multipart 字段 %s 失败: %v", f.name, err)
				return
			}
		}

		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			logError("创建 multipart file 字段失败: %v", err)
			return
		}
		if _, err = io.Copy(part, reader); err != nil {
			logError("写入文件内容至 multipart 失败: %v", err)
			return
		}
		logDebug("multipart 请求体构建完成，Content-Type boundary 已生成")
	}()

	return pr, writer.FormDataContentType(), nil
}

// truncateBody 截断过长的响应体用于日志输出，避免刷屏。
func truncateBody(body []byte) string {
	const maxLen = 512
	s := string(body)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}
