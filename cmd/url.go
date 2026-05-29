package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/oss"
	"github.com/spf13/cobra"
)

var expires int
var thumbnailWidth int
var thumbnailHeight int
var outputFormat string

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

		format := ""
		if strings.EqualFold(outputFormat, "webp") {
			if oss.IsImageKey(objectKey) {
				format = "webp"
			} else {
				log.Printf("[WARN] --format webp 已忽略，非图片文件: %s", objectKey)
			}
		}

		ossClient := oss.GetInstance()
		signedURL, err := ossClient.GetViewSignedURL(objectKey, 0, 0, format, expires)
		if err != nil {
			log.Printf("[ERROR] CLI 生成签名链接失败: %v\n", err)
			log.Fatalf("Failed to generate URL: %v", err)
		}

		log.Printf("[INFO] CLI 成功生成签名链接")
		if format == "webp" {
			fmt.Printf("Generated WebP URL (valid for %d seconds):\n%s\n", expires, signedURL)
		} else {
			fmt.Printf("Generated URL (valid for %d seconds):\n%s\n", expires, signedURL)
		}
	},
}

var thumbnailCmd = &cobra.Command{
	Use:   "thumbnail [object_key]",
	Short: "Generate a temporary thumbnail URL with specified width and height",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("[INFO] 收到 thumbnail 命令请求")
		objectKey := args[0]

		if expires <= 0 {
			expires = config.AppConfig.Server.LinkExpireSeconds
			if expires <= 0 {
				expires = 3600
			}
		}

		if thumbnailWidth <= 0 {
			thumbnailWidth = 200
		}
		if thumbnailHeight <= 0 {
			thumbnailHeight = 80
		}

		log.Printf("[INFO] 准备为文件生成缩略图签名链接，Key: %s，宽度: %dpx，高度: %dpx，有效期: %d 秒", objectKey, thumbnailWidth, thumbnailHeight, expires)

		format := ""
		if strings.EqualFold(outputFormat, "webp") {
			if oss.IsImageKey(objectKey) {
				format = "webp"
			} else {
				log.Printf("[WARN] --format webp 已忽略，非图片文件: %s", objectKey)
			}
		}

		ossClient := oss.GetInstance()
		signedURL, err := ossClient.GetViewSignedURL(objectKey, thumbnailWidth, thumbnailHeight, format, expires)
		if err != nil {
			log.Printf("[ERROR] CLI 生成缩略图签名链接失败: %v\n", err)
			log.Fatalf("Failed to generate thumbnail URL: %v", err)
		}

		log.Printf("[INFO] CLI 成功生成缩略图签名链接")
		if format == "webp" {
			fmt.Printf("Generated Thumbnail WebP URL (width: %dpx, height: %dpx, valid for %d seconds):\n%s\n", thumbnailWidth, thumbnailHeight, expires, signedURL)
		} else {
			fmt.Printf("Generated Thumbnail URL (width: %dpx, height: %dpx, valid for %d seconds):\n%s\n", thumbnailWidth, thumbnailHeight, expires, signedURL)
		}
	},
}

// init 注册 url 命令并配置相关 flag
func init() {
	urlCmd.Flags().IntVarP(&expires, "expires", "e", 0, "Expiration time in seconds")
	urlCmd.Flags().StringVar(&outputFormat, "format", "", "Output image format (webp)")
	rootCmd.AddCommand(urlCmd)

	thumbnailCmd.Flags().IntVarP(&thumbnailWidth, "width", "w", 200, "Thumbnail width in pixels")
	thumbnailCmd.Flags().IntVarP(&thumbnailHeight, "height", "H", 80, "Thumbnail height in pixels")
	thumbnailCmd.Flags().IntVarP(&expires, "expires", "e", 0, "Expiration time in seconds")
	thumbnailCmd.Flags().StringVar(&outputFormat, "format", "", "Output image format (webp)")
	rootCmd.AddCommand(thumbnailCmd)
}