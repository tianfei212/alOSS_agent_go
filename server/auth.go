package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/derekt/oss-cli/config"
	"github.com/gin-gonic/gin"
)

// BlacklistRecord 定义了 IP 黑名单的记录结构
type BlacklistRecord struct {
	IP        string    `json:"ip"`
	FailCount int       `json:"fail_count"`
	BlockedAt time.Time `json:"blocked_at"`
}

var (
	failedAttempts sync.Map
	blacklistedIPs sync.Map
	blacklistFile  = ".blackIP.json"
	blacklistMutex sync.Mutex
)

// InitBlacklist 从 JSON 文件加载黑名单数据
func InitBlacklist() error {
	log.Println("[INFO] 开始加载 IP 黑名单文件...")
	file, err := os.Open(blacklistFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("[INFO] 黑名单文件不存在，将创建新文件")
			return nil
		}
		log.Printf("[ERROR] 打开黑名单文件失败: %v\n", err)
		return err
	}
	defer file.Close()

	var records []BlacklistRecord
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&records); err != nil {
		if err.Error() == "EOF" {
			log.Println("[INFO] 黑名单文件为空")
			return nil
		}
		log.Printf("[ERROR] 解析黑名单 JSON 失败: %v\n", err)
		return err
	}

	for _, record := range records {
		blacklistedIPs.Store(record.IP, record)
	}
	log.Printf("[INFO] 成功加载 %d 条黑名单记录", len(records))
	return nil
}

// saveBlacklistToFile 将内存中的黑名单数据保存到 JSON 文件
func saveBlacklistToFile() {
	blacklistMutex.Lock()
	defer blacklistMutex.Unlock()

	var records []BlacklistRecord
	blacklistedIPs.Range(func(key, value interface{}) bool {
		records = append(records, value.(BlacklistRecord))
		return true
	})

	file, err := os.OpenFile(blacklistFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[ERROR] 无法打开黑名单文件以写入: %v\n", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(records); err != nil {
		log.Printf("[ERROR] 写入黑名单文件失败: %v\n", err)
	} else {
		log.Printf("[INFO] 成功将黑名单保存至 %s", blacklistFile)
	}
}

// addToBlacklist 将 IP 添加到内存黑名单中，并保存到 JSON 文件
func addToBlacklist(ip string, count int) {
	log.Printf("[INFO] 正在将 IP 加入黑名单: %s，错误次数: %d", ip, count)
	record := BlacklistRecord{
		IP:        ip,
		FailCount: count,
		BlockedAt: time.Now(),
	}
	blacklistedIPs.Store(ip, record)
	saveBlacklistToFile()
}

// AuthMiddleware Gin 中间件，用于验证 API Key 并处理 IP 黑名单封禁
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if _, blacklisted := blacklistedIPs.Load(clientIP); blacklisted {
			fmt.Printf("\033[31m[警告] 拦截到黑名单IP请求: %s\033[0m\n", clientIP)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "IP is blacklisted"})
			return
		}

		expectedKey := config.AppConfig.Server.OpenAIAPIKey
		authHeader := c.GetHeader("Authorization")
		var token string
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			token = authHeader
		}

		if expectedKey != "" && token != expectedKey {
			log.Printf("[WARN] API Key 验证失败，IP: %s", clientIP)
			val, _ := failedAttempts.LoadOrStore(clientIP, 0)
			count := val.(int) + 1
			failedAttempts.Store(clientIP, count)

			if count >= 5 {
				log.Printf("[WARN] IP %s 连续验证失败 %d 次，触发黑名单机制", clientIP, count)
				addToBlacklist(clientIP, count)
				failedAttempts.Delete(clientIP)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Too many failed attempts, IP blocked"})
				return
			}

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid API Key"})
			return
		}

		if val, exists := failedAttempts.LoadAndDelete(clientIP); exists {
			log.Printf("[INFO] IP %s 验证成功，已清除之前的 %d 次失败记录", clientIP, val.(int))
		}

		c.Next()
	}
}
