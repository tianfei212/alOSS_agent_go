package cmd

import (
	"log"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/server"
	"github.com/spf13/cobra"
)

var port int

// serverCmd 定义了用于启动 HTTP 服务的命令
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the OpenAI compatible API server",
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("[INFO] 接收到启动 server 的命令")
		// Override port from config if flag is provided
		if port != 0 {
			config.AppConfig.Server.Port = port
		} else if config.AppConfig.Server.Port == 0 {
			config.AppConfig.Server.Port = 8080 // fallback default
		}

		log.Printf("[INFO] 正在启动 API 服务器，端口: %d...", config.AppConfig.Server.Port)
		if err := server.RunServer(); err != nil {
			log.Printf("[ERROR] API 服务器运行失败并退出: %v\n", err)
			log.Fatalf("Server failed: %v", err)
		}
	},
}

// init 注册 server 命令并配置相应的参数 flag
func init() {
	serverCmd.Flags().IntVarP(&port, "port", "p", 0, "Port to run the server on (overrides config file)")
	rootCmd.AddCommand(serverCmd)
}