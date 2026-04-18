package cmd

import (
	"fmt"
	"log"

	"github.com/derekt/oss-cli/oss"
	"github.com/spf13/cobra"
)

var prefix string
var limit int

// listCmd 定义了用于列出 OSS 文件的 CLI 命令
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List files in OSS",
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("[INFO] 收到 list 命令请求")
		log.Printf("[INFO] 准备列出 OSS 文件，前缀: '%s'，数量限制: %d", prefix, limit)
		
		ossClient := oss.GetInstance()
		objects, err := ossClient.ListFiles(prefix, limit)
		if err != nil {
			log.Printf("[ERROR] CLI 获取文件列表失败: %v\n", err)
			log.Fatalf("List files failed: %v", err)
		}

		log.Printf("[INFO] CLI 成功获取 %d 个文件列表", len(objects))
		fmt.Printf("Found %d files:\n", len(objects))
		for _, obj := range objects {
			fmt.Printf("- %s (Size: %d bytes, LastModified: %s)\n", obj.Key, obj.Size, obj.LastModified.Format("2006-01-02 15:04:05"))
		}
	},
}

// init 注册 list 命令并配置相关 flag
func init() {
	listCmd.Flags().StringVarP(&prefix, "prefix", "p", "", "Prefix of the object keys to list")
	listCmd.Flags().IntVarP(&limit, "limit", "l", 100, "Maximum number of files to list")
	rootCmd.AddCommand(listCmd)
}