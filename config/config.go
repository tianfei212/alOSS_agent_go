package config

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config 定义了整个应用的配置结构
type Config struct {
	OSS       OSSConfig       `mapstructure:"oss"`
	Server    ServerConfig    `mapstructure:"server"`
	DashScope DashScopeConfig `mapstructure:"dashscope"`
}

// DashScopeConfig 定义百炼临时文件上传（F5）相关配置。
// 上游凭证来自 .env.local 的 AL_KEY，与 OSS、OPENAI_API_KEY 完全隔离。
type DashScopeConfig struct {
	APIKey       string `mapstructure:"api_key"`
	BaseURL      string `mapstructure:"base_url"`
	DefaultModel string `mapstructure:"default_model"`
}

// OSSConfig 定义了阿里云 OSS 的相关配置
type OSSConfig struct {
	Endpoint              string `mapstructure:"endpoint"`
	AccessKeyID           string `mapstructure:"access_key_id"`
	AccessKeySecret       string `mapstructure:"access_key_secret"`
	BucketName            string `mapstructure:"bucket_name"`
	BucketPrefix          string `mapstructure:"bucket_prefix"`
	DefaultRetentionYears int    `mapstructure:"default_retention_years"`
	AllowedRetentionYears []int  `mapstructure:"allowed_retention_years"`
	AllowedRetentionDays  []int  `mapstructure:"allowed_retention_days"`
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
	viper.BindEnv("oss.default_retention_years", "OSS_DEFAULT_RETENTION_YEARS")
	viper.BindEnv("dashscope.api_key", "AL_KEY")

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

// ValidateDashScopeKey 校验 F5 百炼功能所需的 AL_KEY 是否已配置。
func ValidateDashScopeKey() error {
	if AppConfig.DashScope.APIKey == "" {
		return fmt.Errorf("未配置 AL_KEY，请在 .env.local 中设置客户百炼 API Key")
	}
	return nil
}
