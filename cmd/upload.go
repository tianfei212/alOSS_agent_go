package cmd

import (
	"fmt"
	"log"
	"path"
	"path/filepath"

	"github.com/derekt/oss-cli/oss"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// uploadCmd 定义了用于上传本地文件到 OSS 的 CLI 命令
var uploadCmd = &cobra.Command{
	Use:   "upload [file_path]",
	Short: "Upload a local file to OSS",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("[INFO] 收到 upload 命令请求")
		localPath := args[0]

		absPath, err := filepath.Abs(localPath)
		if err != nil {
			log.Printf("[ERROR] 无效的文件路径 '%s': %v\n", localPath, err)
			log.Fatalf("Invalid path: %v", err)
		}

		fileID := "file-" + uuid.New().String()
		objectKey := fileID + "-" + path.Base(absPath)

		log.Printf("[INFO] 准备上传文件: %s，目标 Key: %s", absPath, objectKey)
		fmt.Printf("Uploading %s to %s...\n", absPath, objectKey)

		ossClient := oss.GetInstance()
		if err := ossClient.UploadFile(absPath, objectKey); err != nil {
			log.Printf("[ERROR] CLI 文件上传失败: %v\n", err)
			log.Fatalf("Upload failed: %v", err)
		}

		log.Printf("[INFO] CLI 文件上传成功，Key: %s", objectKey)
		fmt.Printf("Upload successful. Object Key: %s\n", objectKey)
	},
}

// init 注册 upload 命令
func init() {
	rootCmd.AddCommand(uploadCmd)
}