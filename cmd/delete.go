package cmd

import (
	"fmt"
	"log"

	"github.com/derekt/oss-cli/oss"
	"github.com/spf13/cobra"
)

// deleteCmd 定义了用于从 OSS 删除文件的 CLI 命令
var deleteCmd = &cobra.Command{
	Use:   "delete [object_key]",
	Short: "Delete a file from OSS",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("[INFO] 收到 delete 命令请求")
		objectKey := args[0]

		log.Printf("[INFO] 准备删除 OSS 文件，Key: %s", objectKey)
		fmt.Printf("Deleting %s...\n", objectKey)

		ossClient := oss.GetInstance()
		if err := ossClient.DeleteFile(objectKey); err != nil {
			log.Printf("[ERROR] CLI 删除文件失败: %v\n", err)
			log.Fatalf("Delete failed: %v", err)
		}

		log.Printf("[INFO] CLI 文件删除成功，Key: %s", objectKey)
		fmt.Printf("Deleted %s successfully.\n", objectKey)
	},
}

// init 注册 delete 命令
func init() {
	rootCmd.AddCommand(deleteCmd)
}