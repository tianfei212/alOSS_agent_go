package dashscope

import "log"

// logInfo 输出 INFO 级别日志，记录百炼上传流程中的关键步骤。
func logInfo(format string, args ...interface{}) {
	log.Printf("[INFO] dashscope: "+format, args...)
}

// logDebug 输出 DEBUG 级别日志，记录请求细节与中间状态（不含密钥明文）。
func logDebug(format string, args ...interface{}) {
	log.Printf("[DEBUG] dashscope: "+format, args...)
}

// logError 输出 ERROR 级别日志，记录失败原因与上下文。
func logError(format string, args ...interface{}) {
	log.Printf("[ERROR] dashscope: "+format, args...)
}
