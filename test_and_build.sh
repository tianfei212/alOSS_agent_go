#!/bin/bash
set -e

echo "[1/4] Overwriting config.go and auth.go to bypass IDE auto-save..."

cat << 'CONFIG_EOF' > config/config.go
package config

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 定义了整个应用的配置结构
type Config struct {
	OSS    OSSConfig    `mapstructure:"oss"`
	Server ServerConfig `mapstructure:"server"`
}

// OSSConfig 定义了阿里云 OSS 的相关配置
type OSSConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	AccessKeySecret string `mapstructure:"access_key_secret"`
	BucketName      string `mapstructure:"bucket_name"`
	BucketPrefix    string `mapstructure:"bucket_prefix"`
}

// ServerConfig 定义了 HTTP 服务的相关配置
type ServerConfig struct {
	Port              int    `mapstructure:"port"`
	LinkExpireSeconds int    `mapstructure:"link_expire_seconds"`
	OpenAIAPIKey      string `mapstructure:"openai_api_key"`
}

// AppConfig 全局配置变量
var AppConfig Config

// LoadConfig 从指定的文件或环境变量中加载配置
func LoadConfig(cfgFile string) error {
	log.Println("[INFO] 开始加载配置文件和环境变量...")
	if err := godotenv.Load(".env.local"); err != nil {
		log.Println("[INFO] 未找到 .env.local 文件或加载失败，将继续使用环境变量")
	} else {
		log.Println("[INFO] 成功加载 .env.local 环境变量文件")
	}

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
	}

	viper.AutomaticEnv()

	viper.BindEnv("server.openai_api_key", "OPENAI_API_KEY")
	viper.BindEnv("oss.endpoint", "OSS_ENDPOINT")
	viper.BindEnv("oss.access_key_id", "OSS_ACCESS_KEY_ID")
	viper.BindEnv("oss.access_key_secret", "OSS_ACCESS_KEY_SECRET")
	viper.BindEnv("oss.bucket_name", "OSS_BUCKET")
	viper.BindEnv("oss.bucket_prefix", "OSS_BUCKET_PREFIX")

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("[ERROR] 读取配置文件失败: %v\n", err)
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Printf("[ERROR] 解析配置数据失败: %v\n", err)
		return fmt.Errorf("解析配置数据失败: %w", err)
	}

	log.Println("[INFO] 配置加载成功")
	return nil
}
CONFIG_EOF

cat << 'AUTH_EOF' > server/auth.go
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
			log.Printf("[WARN] API Key 验证失败，IP: %s, 提供的 Token: %s", clientIP, token)
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
AUTH_EOF

echo "[2/4] Building oss-cli locally..."
go build -o oss-cli .

echo "[3/4] HTTP API Tests"
./oss-cli server -p 8084 &
SERVER_PID=$!
sleep 3

TOKEN=$(grep OPENAI_API_KEY .env.local | cut -d '=' -f 2)

# Use a generated dummy file for upload test (avoids uploading huge real files every time)
echo "-> Generating 50MB test file..."
head -c 52428800 /dev/urandom > /tmp/test_upload.bin

echo "-> Testing HTTP Upload (50MB file to simulate real frontend upload)"
UPLOAD_RES=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" -F "file=@/tmp/test_upload.bin" http://127.0.0.1:8084/v1/files)
echo $UPLOAD_RES
FILE_ID=$(echo $UPLOAD_RES | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -n "$FILE_ID" ]; then
    echo "-> HTTP Get Info"
    curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8084/v1/files/$FILE_ID
    echo ""

    echo "-> HTTP Get Content (Download URL)"
    curl -s -I -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8084/v1/files/$FILE_ID/content | head -n 1
    echo ""

    echo "-> Deleting test file after test..."
    DELETE_RES=$(curl -s -X DELETE -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8084/v1/files/$FILE_ID)
    echo $DELETE_RES
    echo ""
fi

echo "-> Stress Test (50 requests/min for 1 minute)..."
START_TIME=$(date +%s)
COUNT=0
while true; do
  CUR_TIME=$(date +%s)
  ELAPSED=$((CUR_TIME - START_TIME))
  if [ $ELAPSED -ge 60 ]; then
    break
  fi
  curl -s -o /dev/null -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8084/v1/files &
  COUNT=$((COUNT + 1))
  sleep 1.2
done
wait
echo "Stress test complete. Sent $COUNT requests in 60 seconds."

kill -9 $SERVER_PID || true

echo "[4/4] Building Cross-Platform Binaries and Setup Script..."
mkdir -p dist
GOOS=darwin GOARCH=arm64 go build -o dist/oss-cli-darwin-arm64 .
GOOS=linux GOARCH=amd64 go build -o dist/oss-cli-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o dist/oss-cli-linux-arm64 .

cat << 'SETUP_EOF' > setup.sh
#!/bin/bash

REPO="derekt/oss-cli"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" = "x86_64" ]; then
    ARCH="amd64"
elif [ "$ARCH" = "aarch64" ]; then
    ARCH="arm64"
fi

BIN_NAME="oss-cli-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/latest/download/${BIN_NAME}"

echo "Downloading ${BIN_NAME} from GitHub..."
curl -L -o oss-cli ${URL}
chmod +x oss-cli
echo "Installed successfully. Run ./oss-cli to start."
SETUP_EOF
chmod +x setup.sh

echo "All tasks completed successfully!"
