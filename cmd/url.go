package cmd

import (
	"fmt"
	"log"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/oss"
	"github.com/spf13/cobra"
)

var expires int

// urlCmd 定义了生成临时下载链接的 CLI 命令
var urlCmd = &cobra.Command{
	Use:   "url [object_key]",
	Short: "Generate a temporary download/play URL for a file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("[INFO] 收到 url 命令请求")
		objectKey := args[0]

		if expires <= 0 {
			expires = config.AppConfig.Server.LinkExpireSeconds
			if expires <= 0 {
				expires = 3600
			}
		}

		log.Printf("[INFO] 准备为文件生成签名链接，Key: %s，有效期: %d 秒", objectKey, expires)

		ossClient := oss.GetInstance()
		signedURL, err := ossClient.GetSignedURL(objectKey, expires)
		if err != nil {
			log.Printf("[ERROR] CLI 生成签名链接失败: %v\n", err)
			log.Fatalf("Failed to generate URL: %v", err)
		}

		log.Printf("[INFO] CLI 成功生成签名链接")
		fmt.Printf("Generated URL (valid for %d seconds):\n%s\n", expires, signedURL)
	},
}

// init 注册 url 命令并配置相关 flag
func init() {
	urlCmd.Flags().IntVarP(&expires, "expires", "e", 0, "Expiration time in seconds")
	rootCmd.AddCommand(urlCmd)
}