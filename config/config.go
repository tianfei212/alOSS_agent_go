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
