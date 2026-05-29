package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/dashscope"
	"github.com/spf13/cobra"
)

// dashscopeCmd 百炼临时文件上传子命令组（F5），不依赖 OSS 配置。
var dashscopeCmd = &cobra.Command{
	Use:   "dashscope",
	Short: "百炼临时文件上传（F5，使用 AL_KEY，无需 OSS 配置）",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		log.Println("[INFO] dashscope 子命令：仅加载配置并校验 AL_KEY，跳过 OSS 初始化")
		if err := config.ValidateDashScopeKey(); err != nil {
			log.Printf("[ERROR] %v", err)
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		log.Printf("[DEBUG] AL_KEY 已配置，base_url=%s",
			config.AppConfig.DashScope.BaseURL)
	},
}

// dashscopeUploadCmd 端到端上传本地文件并输出 oss_url。
var dashscopeUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "上传本地文件至百炼临时存储并获取 oss:// URL",
	Run: func(cmd *cobra.Command, args []string) {
		model, _ := cmd.Flags().GetString("model")
		filePath, _ := cmd.Flags().GetString("file")

		if model == "" {
			log.Println("[ERROR] 缺少 --model 参数")
			fmt.Fprintln(os.Stderr, "错误: 必须指定 --model")
			os.Exit(1)
		}
		if filePath == "" {
			log.Println("[ERROR] 缺少 --file 参数")
			fmt.Fprintln(os.Stderr, "错误: 必须指定 --file")
			os.Exit(1)
		}

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			log.Printf("[ERROR] 无效文件路径: %v", err)
			fmt.Fprintf(os.Stderr, "错误: 无效文件路径: %v\n", err)
			os.Exit(1)
		}

		if _, err := os.Stat(absPath); err != nil {
			log.Printf("[ERROR] 文件不存在: %s", absPath)
			fmt.Fprintf(os.Stderr, "错误: 文件不存在: %s\n", absPath)
			os.Exit(1)
		}

		log.Printf("[INFO] 开始百炼上传，model=%s，file=%s", model, absPath)

		client := dashscope.NewClient(
			config.AppConfig.DashScope.APIKey,
			config.AppConfig.DashScope.BaseURL,
		)

		openFile := func() (io.ReadCloser, int64, error) {
			f, err := os.Open(absPath)
			if err != nil {
				return nil, 0, err
			}
			info, err := f.Stat()
			if err != nil {
				f.Close()
				return nil, 0, err
			}
			return f, info.Size(), nil
		}

		result, err := client.UploadAndGetURL(cmd.Context(), model, filepath.Base(absPath), openFile)
		if err != nil {
			log.Printf("[ERROR] 百炼上传失败: %v", err)
			fmt.Fprintf(os.Stderr, "上传失败: %v\n", err)
			os.Exit(2)
		}

		log.Printf("[INFO] 上传成功，oss_url=%s", result.OSSURL)
		fmt.Printf("oss_url: %s\n", result.OSSURL)
		fmt.Printf("expires_at: %s\n", result.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"))
		fmt.Printf("model: %s\n", result.Model)
		fmt.Printf("filename: %s\n", result.Filename)
		fmt.Println("提示: 调用百炼模型时请在 Header 中添加 X-DashScope-OssResourceResolve: enable")
	},
}

// dashscopePolicyCmd 仅获取 getPolicy 凭证并输出 JSON。
var dashscopePolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "获取百炼上传凭证（getPolicy）",
	Run: func(cmd *cobra.Command, args []string) {
		model, _ := cmd.Flags().GetString("model")
		if model == "" {
			log.Println("[ERROR] 缺少 --model 参数")
			fmt.Fprintln(os.Stderr, "错误: 必须指定 --model")
			os.Exit(1)
		}

		log.Printf("[INFO] 请求 getPolicy，model=%s", model)

		client := dashscope.NewClient(
			config.AppConfig.DashScope.APIKey,
			config.AppConfig.DashScope.BaseURL,
		)

		policy, err := client.GetUploadPolicy(cmd.Context(), model)
		if err != nil {
			log.Printf("[ERROR] getPolicy 失败: %v", err)
			fmt.Fprintf(os.Stderr, "getPolicy 失败: %v\n", err)
			os.Exit(2)
		}

		out, err := json.MarshalIndent(policy, "", "  ")
		if err != nil {
			log.Printf("[ERROR] JSON 序列化失败: %v", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	},
}

func init() {
	dashscopeUploadCmd.Flags().String("model", "", "百炼模型名称，如 qwen-vl-plus")
	dashscopeUploadCmd.Flags().String("file", "", "待上传的本地文件路径")
	_ = dashscopeUploadCmd.MarkFlagRequired("model")
	_ = dashscopeUploadCmd.MarkFlagRequired("file")

	dashscopePolicyCmd.Flags().String("model", "", "百炼模型名称，如 qwen-vl-plus")
	_ = dashscopePolicyCmd.MarkFlagRequired("model")

	dashscopeCmd.AddCommand(dashscopeUploadCmd)
	dashscopeCmd.AddCommand(dashscopePolicyCmd)
	rootCmd.AddCommand(dashscopeCmd)
}
