package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/oss"
	"github.com/gin-gonic/gin"
)

func stripPrefix(key string) string {
	prefix := config.AppConfig.OSS.BucketPrefix
	if prefix != "" {
		key = strings.TrimPrefix(key, prefix)
	}
	return key
}

func ossKey(key string) string {
	if config.AppConfig.OSS.BucketPrefix != "" {
		return path.Join(config.AppConfig.OSS.BucketPrefix, key)
	}
	return key
}

// RunServer 启动兼容 OpenAI 接口规范的 HTTP API 服务
func RunServer() error {
	log.Println("[INFO] 准备启动 HTTP API 服务...")
	// 初始化 IP 黑名单
	if err := InitBlacklist(); err != nil {
		log.Printf("[ERROR] 初始化 IP 黑名单失败: %v\n", err)
	}

	// 强制 gin 日志打印到标准输出
	gin.DefaultWriter = os.Stdout

	r := gin.Default()

	// 允许跨域请求 (CORS) 方便前端调用，需要在全局设置以捕获所有 OPTIONS 请求
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Not Found"})
	})

	v1 := r.Group("/v1")
	{
		// 使用鉴权与黑名单中间件
		v1.Use(AuthMiddleware())

		// https://platform.openai.com/docs/api-reference/files
		v1.POST("/files", uploadFile)
		v1.GET("/files", listFiles)
		v1.GET("/files/:file_id", getFileInfo)
		v1.DELETE("/files/:file_id", deleteFile)
		v1.GET("/files/:file_id/content", getFileContent)
	}

	// 增加专用于展示视频或图片的页面接口
	r.GET("/view/*file_id", viewMedia)

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	log.Printf("[INFO] 启动服务，监听地址: %s", addr)
	return r.Run(addr)
}

// uploadFile 处理文件上传请求，支持表单上传和 URL 离线下载上传
func uploadFile(c *gin.Context) {
	log.Println("[INFO] 收到上传文件请求")
	// 首先尝试从 multipart form 获取上传的文件
	file, err := c.FormFile("file")

	// 支持如果没传文件，而是传了 file_url 的情况（服务端代为下载并上传到 OSS）
	fileURL := c.PostForm("file_url")

	if err != nil && fileURL == "" {
		log.Println("[WARN] 请求中缺少 file 或 file_url 参数")
		c.JSON(http.StatusBadRequest, gin.H{"error": "File or file_url is required"})
		return
	}

	purpose := c.PostForm("purpose")
	if purpose == "" {
		purpose = "fine-tune" // default or mock
	}

	var src io.ReadCloser
	var filename string
	var fileSize int64

	if file != nil {
		log.Printf("[INFO] 接收到本地上传文件: %s，大小: %d 字节", file.Filename, file.Size)
		src, err = file.Open()
		if err != nil {
			log.Printf("[ERROR] 打开上传文件失败: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open uploaded file"})
			return
		}
		defer src.Close()
		filename = file.Filename
		fileSize = file.Size
	} else if fileURL != "" {
		log.Printf("[INFO] 接收到基于 URL 的上传请求，开始下载: %s", fileURL)
		resp, err := http.Get(fileURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("[ERROR] 从提供的 URL 获取文件失败: %v, HTTP 状态码: %d\n", err, resp.StatusCode)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to fetch file from provided URL"})
			if resp != nil {
				resp.Body.Close()
			}
			return
		}
		src = resp.Body
		defer src.Close()
		// 从 URL 解析出文件名
		filename = "downloaded_file"
		for i := len(fileURL) - 1; i >= 0; i-- {
			if fileURL[i] == '/' {
				if i < len(fileURL)-1 {
					filename = fileURL[i+1:]
				}
				break
			}
		}
		fileSize = resp.ContentLength
		log.Printf("[INFO] 远程文件下载流准备完毕，文件名: %s，大小: %d 字节", filename, fileSize)
	}

	objectKey := filename

	ossClient := oss.GetInstance()
	log.Printf("[INFO] 开始流式上传至 OSS，键名: %s", objectKey)
	if err := ossClient.UploadStream(src, objectKey); err != nil {
		log.Printf("[ERROR] 上传至 OSS 失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	expireSec := config.AppConfig.Server.LinkExpireSeconds
	if expireSec == 0 {
		expireSec = 3600
	}

	signedURL, err := ossClient.GetSignedURL(objectKey, expireSec)
	if err != nil {
		log.Printf("[ERROR] 生成上传后预览签名链接失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[INFO] 文件上传处理完成，返回文件信息")
	c.JSON(http.StatusOK, gin.H{
		"id":         objectKey, // use objectKey as ID to simplify
		"object":     "file",
		"bytes":      fileSize,
		"created_at": time.Now().Unix(),
		"filename":   filename,
		"purpose":    purpose,
		"view_url":   signedURL,
	})
}

// listFiles 获取 OSS 上的文件列表并以 JSON 格式返回
func listFiles(c *gin.Context) {
	log.Println("[INFO] 收到获取文件列表请求")
	ossClient := oss.GetInstance()
	objects, err := ossClient.ListFiles("", 100)
	if err != nil {
		log.Printf("[ERROR] 获取文件列表失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	expireSec := config.AppConfig.Server.LinkExpireSeconds
	if expireSec == 0 {
		expireSec = 3600
	}

	var data []map[string]interface{}
	for _, obj := range objects {
		publicKey := stripPrefix(obj.Key)
		if publicKey == "" || strings.HasSuffix(obj.Key, "/") {
			continue
		}

		signedURL, err := ossClient.GetSignedURL(publicKey, expireSec)
		if err != nil {
			log.Printf("[ERROR] 为列表项生成签名链接失败，文件: %s, err: %v\n", obj.Key, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		data = append(data, map[string]interface{}{
			"id":         publicKey,
			"object":     "file",
			"bytes":      obj.Size,
			"created_at": obj.LastModified.Unix(),
			"filename":   publicKey,
			"purpose":    "assistants", // mock
			"view_url":   signedURL,
		})
	}

	log.Printf("[INFO] 成功返回 %d 个文件的列表", len(data))
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   data,
	})
}

// getFileInfo 获取指定文件的详细元数据信息
func getFileInfo(c *gin.Context) {
	fileID := c.Param("file_id")
	log.Printf("[INFO] 收到获取文件信息请求，文件ID: %s", fileID)

	ossClient := oss.GetInstance()
	obj, err := ossClient.GetFileInfo(stripPrefix(fileID))
	if err != nil {
		log.Printf("[ERROR] 文件信息获取失败，文件可能不存在: %v\n", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	expireSec := config.AppConfig.Server.LinkExpireSeconds
	if expireSec == 0 {
		expireSec = 3600
	}

	signedURL, err := ossClient.GetSignedURL(stripPrefix(fileID), expireSec)
	if err != nil {
		log.Printf("[ERROR] 生成文件预览签名链接失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[INFO] 成功获取文件信息，文件名: %s，大小: %d", obj.Key, obj.Size)
	publicKey := stripPrefix(obj.Key)
	c.JSON(http.StatusOK, gin.H{
		"id":         publicKey,
		"object":     "file",
		"bytes":      obj.Size,
		"created_at": obj.LastModified.Unix(),
		"filename":   publicKey,
		"purpose":    "assistants", // mock
		"view_url":   signedURL,
	})
}

// deleteFile 处理文件删除请求
func deleteFile(c *gin.Context) {
	fileID := c.Param("file_id")
	log.Printf("[INFO] 收到删除文件请求，文件ID: %s", fileID)

	ossClient := oss.GetInstance()
	if err := ossClient.DeleteFile(stripPrefix(fileID)); err != nil {
		log.Printf("[ERROR] 删除文件失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[INFO] 成功删除文件: %s", fileID)
	c.JSON(http.StatusOK, gin.H{
		"id":      stripPrefix(fileID),
		"object":  "file",
		"deleted": true,
	})
}

// getFileContent 生成文件下载链接并进行临时重定向
func getFileContent(c *gin.Context) {
	fileID := c.Param("file_id")
	expireSecStr := c.Query("expire_seconds")
	log.Printf("[INFO] 收到获取文件内容下载链接请求，文件ID: %s", fileID)

	expireSec := config.AppConfig.Server.LinkExpireSeconds
	if expireSec == 0 {
		expireSec = 3600
	}

	if expireSecStr != "" {
		if val, err := strconv.Atoi(expireSecStr); err == nil && val > 0 {
			expireSec = val
		}
	}

	ossClient := oss.GetInstance()
	signedURL, err := ossClient.GetSignedURL(stripPrefix(fileID), expireSec)
	if err != nil {
		log.Printf("[ERROR] 生成文件下载签名链接失败: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[INFO] 成功生成签名下载链接，将重定向客户端 (有效期 %d 秒)", expireSec)
	// Redirect to the temporary download/play link
	c.Redirect(http.StatusTemporaryRedirect, signedURL)
}

// viewMedia 返回 OSS 签名 URL（JSON 格式），供前端直接播放/展示，无需鉴权
// 支持 query 参数 w（宽度）和 h（高度）生成缩略图
func viewMedia(c *gin.Context) {
	fileID := strings.TrimPrefix(c.Param("file_id"), "/")
	expireSecStr := c.Query("expire_seconds")
	widthStr := c.Query("w")
	heightStr := c.Query("h")
	log.Printf("[INFO] 收到媒体预览请求，文件ID: %s，宽度: %spx，高度: %spx", fileID, widthStr, heightStr)

	expireSec := config.AppConfig.Server.LinkExpireSeconds
	if expireSec == 0 {
		expireSec = 3600
	}

	if expireSecStr != "" {
		if val, err := strconv.Atoi(expireSecStr); err == nil && val > 0 {
			expireSec = val
		}
	}

	ossClient := oss.GetInstance()
	_, err := ossClient.GetFileInfo(stripPrefix(fileID))
	if err != nil {
		log.Printf("[ERROR] 预览请求失败，文件在 OSS 上不存在: %v\n", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found on OSS"})
		return
	}

	var signedURL string
	width, wErr := strconv.Atoi(widthStr)
	height, hErr := strconv.Atoi(heightStr)
	if wErr == nil && hErr == nil && width > 0 && height > 0 {
		signedURL, err = ossClient.GetThumbnailSignedURL(stripPrefix(fileID), width, height, expireSec)
		if err != nil {
			log.Printf("[ERROR] 生成缩略图签名链接失败: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[INFO] 使用缩略图模式，宽度: %dpx，高度: %dpx", width, height)
	} else {
		signedURL, err = ossClient.GetSignedURL(stripPrefix(fileID), expireSec)
		if err != nil {
			log.Printf("[ERROR] 生成签名链接失败: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	ext := ""
	for i := len(fileID) - 1; i >= 0; i-- {
		if fileID[i] == '.' {
			ext = strings.ToLower(fileID[i:])
			break
		}
	}

	mediaType := "file"
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		mediaType = "image"
	case ".mp4", ".webm", ".ogg", ".mov", ".mkv":
		mediaType = "video"
	case ".mp3", ".wav", ".flac", ".aac", ".m4a":
		mediaType = "audio"
	case ".pdf":
		mediaType = "pdf"
	}

	log.Printf("[INFO] 返回媒体预览信息，类型: %s，签名URL有效期: %d秒", mediaType, expireSec)
	c.JSON(http.StatusOK, gin.H{
		"id":         stripPrefix(fileID),
		"media_type": mediaType,
		"url":        signedURL,
		"expires_in": expireSec,
		"created_at": time.Now().Unix(),
	})
}
