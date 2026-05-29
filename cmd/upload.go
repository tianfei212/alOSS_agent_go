package cmd

import (
	"fmt"
	"log"
	"path"
	"path/filepath"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/oss"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var uploadRetentionYears int
var uploadRetentionDays int

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

		var policy oss.RetentionPolicy
		if uploadRetentionDays > 0 {
			if err := oss.ValidateRetentionDays(uploadRetentionDays, config.MaxRetentionDays); err != nil {
				log.Fatalf("Invalid retention days: %v", err)
			}
			if !config.IsAllowedRetentionDays(uploadRetentionDays) {
				log.Fatalf("retention_days %d not in allowed list %v", uploadRetentionDays, config.AllowedRetentionDays())
			}
			policy = oss.RetentionDays(uploadRetentionDays)
		} else {
			retentionYears := uploadRetentionYears
			if retentionYears <= 0 {
				retentionYears = config.DefaultRetentionYears()
			}
			if err := oss.ValidateRetentionYears(retentionYears, config.MaxRetentionYears); err != nil {
				log.Fatalf("Invalid retention years: %v", err)
			}
			if !config.IsAllowedRetentionYears(retentionYears) {
				log.Fatalf("retention_years %d not in allowed list %v", retentionYears, config.AllowedRetentionYears())
			}
			policy = oss.RetentionYears(retentionYears)
		}

		fileID := "file-" + uuid.New().String()
		objectKey := fileID + "-" + path.Base(absPath)

		log.Printf("[INFO] 准备上传文件: %s，目标 Key: %s，保存周期: %s", absPath, objectKey, policy.String())
		fmt.Printf("Uploading %s to %s (retention: %s)...\n", absPath, objectKey, policy.String())

		ossClient := oss.GetInstance()
		if err := ossClient.UploadFile(absPath, objectKey, policy); err != nil {
			log.Printf("[ERROR] CLI 文件上传失败: %v\n", err)
			log.Fatalf("Upload failed: %v", err)
		}

		log.Printf("[INFO] CLI 文件上传成功，Key: %s", objectKey)
		fmt.Printf("Upload successful. Object Key: %s\n", objectKey)
	},
}

func init() {
	uploadCmd.Flags().IntVar(&uploadRetentionYears, "retention-years", 0,
		"Retention period in years (default from config, usually 2)")
	uploadCmd.Flags().IntVar(&uploadRetentionDays, "retention-days", 0,
		"Retention period in days for short-term/test uploads (OSS min 1 day)")
	rootCmd.AddCommand(uploadCmd)
}
