package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/oss"
	"github.com/spf13/cobra"
)

var cfgFile string
var appVersion = "V1.0.1"

// rootCmd 是整个 CLI 应用程序的基础命令
var rootCmd = &cobra.Command{
	Use:     "oss-cli",
	Short:   "A CLI tool and API server for Aliyun OSS",
	Version: appVersion,
}

// Execute 是整个命令行工具的入口函数
func Execute() {
	log.Println("[INFO] 开始执行 CLI 命令...")
	if err := rootCmd.Execute(); err != nil {
		log.Printf("[ERROR] CLI 执行失败: %v\n", err)
		fmt.Println(err)
		os.Exit(1)
	}
}

// init 初始化命令行参数，包括配置文件路径
func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
}

// initConfig 在执行命令前，初始化并加载配置文件和 OSS 客户端
func initConfig() {
	log.Println("[INFO] 初始化配置和 OSS 客户端...")
	if err := config.LoadConfig(cfgFile); err != nil {
		log.Printf("[ERROR] 加载配置文件出错: %v\n", err)
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	if err := oss.Init(config.AppConfig.OSS); err != nil {
		log.Printf("[ERROR] 初始化 OSS 客户端失败: %v\n", err)
		fmt.Printf("Error initializing OSS client: %v\n", err)
		os.Exit(1)
	}
	log.Println("[INFO] CLI 初始化完成")
}
