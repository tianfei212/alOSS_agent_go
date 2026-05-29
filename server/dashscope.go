package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/dashscope"
	"github.com/gin-gonic/gin"
)

// dashscopeClient 返回配置好的百炼临时上传客户端。
func dashscopeClient() *dashscope.Client {
	return dashscope.NewClient(
		config.AppConfig.DashScope.APIKey,
		config.AppConfig.DashScope.BaseURL,
	)
}

// getDashScopeUploadPolicy 处理 GET /v1/dashscope/uploads?action=getPolicy&model=。
// 代理百炼 getPolicy 接口，透传官方响应结构。
func getDashScopeUploadPolicy(c *gin.Context) {
	log.Println("[INFO] 收到百炼 getPolicy 请求")

	action := c.Query("action")
	if action != "getPolicy" {
		log.Printf("[WARN] 无效的 action 参数: %s", action)
		c.JSON(http.StatusBadRequest, gin.H{"error": "action 必须为 getPolicy"})
		return
	}

	model := c.Query("model")
	if model == "" {
		log.Println("[WARN] getPolicy 请求缺少 model 参数")
		c.JSON(http.StatusBadRequest, gin.H{"error": "model 参数不能为空"})
		return
	}

	log.Printf("[DEBUG] getPolicy 参数: model=%s", model)

	client := dashscopeClient()
	policy, err := client.GetUploadPolicy(c.Request.Context(), model)
	if err != nil {
		writeDashScopeUpstreamError(c, err)
		return
	}

	log.Printf("[INFO] getPolicy 成功，request_id=%s", policy.RequestID)
	c.JSON(http.StatusOK, policy)
}

// postDashScopeUpload 处理 POST /v1/dashscope/uploads 一站式上传。
// multipart 表单字段：model（必填）、file（必填）。
func postDashScopeUpload(c *gin.Context) {
	log.Println("[INFO] 收到百炼一站式上传请求")

	model := c.PostForm("model")
	if model == "" {
		log.Println("[WARN] 上传请求缺少 model 参数")
		c.JSON(http.StatusBadRequest, gin.H{"error": "model 参数不能为空"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		log.Printf("[WARN] 上传请求缺少 file 字段: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "file 字段不能为空"})
		return
	}

	log.Printf("[INFO] 上传文件: %s，大小: %d 字节，model: %s",
		fileHeader.Filename, fileHeader.Size, model)

	tmpFile, err := os.CreateTemp("", "dashscope-upload-*")
	if err != nil {
		log.Printf("[ERROR] 创建临时文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "内部错误：无法创建临时文件"})
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	src, err := fileHeader.Open()
	if err != nil {
		tmpFile.Close()
		log.Printf("[ERROR] 打开上传文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取上传文件"})
		return
	}

	if _, err := io.Copy(tmpFile, src); err != nil {
		src.Close()
		tmpFile.Close()
		log.Printf("[ERROR] 写入临时文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法保存上传文件"})
		return
	}
	src.Close()
	tmpFile.Close()

	openFile := func() (io.ReadCloser, int64, error) {
		f, err := os.Open(tmpPath)
		if err != nil {
			return nil, 0, err
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, 0, err
		}
		return f, info.Size(), nil
	}

	client := dashscopeClient()
	result, err := client.UploadAndGetURL(c.Request.Context(), model, fileHeader.Filename, openFile)
	if err != nil {
		writeDashScopeUpstreamError(c, err)
		return
	}

	log.Printf("[INFO] 百炼上传成功，oss_url=%s，expires_at=%s",
		result.OSSURL, result.ExpiresAt.Format(time.RFC3339))

	c.JSON(http.StatusOK, gin.H{
		"oss_url":    result.OSSURL,
		"expires_at": result.ExpiresAt.Format(time.RFC3339),
		"model":      result.Model,
		"filename":   result.Filename,
	})
}

// writeDashScopeUpstreamError 将百炼上游错误映射为合适的 HTTP 状态码与 JSON 响应。
func writeDashScopeUpstreamError(c *gin.Context, err error) {
	if apiErr, ok := err.(*dashscope.APIError); ok {
		status := apiErr.StatusCode
		if status < 400 || status > 599 {
			status = http.StatusBadGateway
		}
		log.Printf("[ERROR] 百炼上游错误，HTTP %d: %s", status, apiErr.Error())

		var body map[string]interface{}
		if json.Unmarshal([]byte(apiErr.Body), &body) == nil && len(body) > 0 {
			c.JSON(status, body)
			return
		}
		c.JSON(status, gin.H{"error": apiErr.Error()})
		return
	}

	log.Printf("[ERROR] 百炼上传处理失败: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}

// registerDashScopeRoutes 注册 F5 百炼临时上传路由（独立于 /v1/files）。
func registerDashScopeRoutes(r *gin.Engine) {
	ds := r.Group("/v1/dashscope")
	ds.Use(DashScopeAuthMiddleware())
	{
		ds.GET("/uploads", getDashScopeUploadPolicy)
		ds.POST("/uploads", postDashScopeUpload)
	}
	log.Println("[INFO] 已注册 F5 百炼路由: /v1/dashscope/uploads")
}
