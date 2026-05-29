// 列出 OSS 全部文件并输出 Markdown 表格（含标签、生命周期与预计删除时间）。
//
// 用法: go run ./scripts/oss-inventory/ --config config.yaml --out docs/OSS-FILE-INVENTORY.md
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/derekt/oss-cli/config"
	osscli "github.com/derekt/oss-cli/oss"
)

func stripPrefix(key, prefix string) string {
	if prefix == "" {
		return key
	}
	p := strings.TrimSuffix(prefix, "/")
	if strings.HasPrefix(key, p+"/") {
		return strings.TrimPrefix(key, p+"/")
	}
	if strings.HasPrefix(key, p) {
		return strings.TrimPrefix(key, p)
	}
	return key
}

func formatSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "已到期/待删除"
	}
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%d天%d小时", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时", hours)
	}
	mins := int(d.Minutes())
	if mins > 0 {
		return fmt.Sprintf("%d分钟", mins)
	}
	return "不足1分钟"
}

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	outPath := flag.String("out", "", "output markdown file (default stdout)")
	listLimit := flag.Int("list-limit", 1000, "max objects per OSS list page (1-1000)")
	flag.Parse()

	if err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := osscli.Init(config.AppConfig.OSS); err != nil {
		log.Fatalf("init oss: %v", err)
	}

	client := osscli.GetInstance()
	bucketPrefix := config.AppConfig.OSS.BucketPrefix

	lifecycleRules, err := client.GetBucketLifecycleRules()
	if err != nil {
		log.Fatalf("get lifecycle: %v", err)
	}
	hasLifecycleConfig := len(lifecycleRules) > 0
	log.Printf("[INFO] 获取到 %d 条生命周期规则", len(lifecycleRules))

	type row struct {
		time       time.Time
		name       string
		size       int64
		tagLabel   string
		lifecycle  string
		remaining  string
		deleteAt   time.Time
	}
	var rows []row
	var continuation string
	now := time.Now()

	for {
		pageLimit := *listLimit
		if pageLimit < 1 {
			pageLimit = 1000
		}
		if pageLimit > 1000 {
			pageLimit = 1000
		}

		objects, next, err := client.ListFilesPage("", pageLimit, continuation)
		if err != nil {
			log.Fatalf("list files: %v", err)
		}
		for _, obj := range objects {
			publicKey := stripPrefix(obj.Key, strings.TrimSuffix(bucketPrefix, "/"))
			if publicKey == "" || strings.HasSuffix(obj.Key, "/") {
				continue
			}

			tagRes, tagErr := client.Bucket.GetObjectTagging(obj.Key)
			var objectTags []oss.Tag
			tagLabel := "—"
			if tagErr == nil {
				objectTags = tagRes.Tags
				if p, ok := osscli.ParseRetentionPolicy(objectTags); ok {
					tagLabel = p.String()
				}
			}

			lc := osscli.MatchLifecycleForObject(obj.Key, objectTags, lifecycleRules)
			remaining := "—"
			var deleteAt time.Time
			if !hasLifecycleConfig {
				lc.Label = "Bucket 未配置生命周期"
				remaining = "永久保留"
			} else if lc.ExpireDays > 0 {
				deleteAt = obj.LastModified.Add(time.Duration(lc.ExpireDays) * 24 * time.Hour)
				remaining = formatDuration(deleteAt.Sub(now))
			} else if lc.Label == "无匹配规则" || strings.Contains(lc.Label, "无匹配 Lifecycle") {
				remaining = "永久保留"
			}

			rows = append(rows, row{
				time:      obj.LastModified,
				name:      publicKey,
				size:      obj.Size,
				tagLabel:  tagLabel,
				lifecycle: lc.Label,
				remaining: remaining,
				deleteAt:  deleteAt,
			})
		}
		if next == "" {
			break
		}
		continuation = next
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].time.After(rows[j].time)
	})

	var b strings.Builder
	b.WriteString("# OSS 文件清单\n\n")
	b.WriteString(fmt.Sprintf("> 生成时间: %s  \n", now.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("> Bucket: `%s`  \n", config.AppConfig.OSS.BucketName))
	b.WriteString(fmt.Sprintf("> 前缀: `%s`  \n", bucketPrefix))
	b.WriteString(fmt.Sprintf("> 文件总数: **%d**\n\n", len(rows)))

	totalSize := int64(0)
	tag3Count, tag2Count, noTagCount := 0, 0, 0
	for _, r := range rows {
		totalSize += r.size
		switch r.tagLabel {
		case "3 年":
			tag3Count++
		case "2 年":
			tag2Count++
		case "—":
			noTagCount++
		}
	}
	b.WriteString(fmt.Sprintf("> 总大小: **%s**\n\n", formatSize(totalSize)))
	b.WriteString(fmt.Sprintf("> 保存周期统计: **3年** %d · **2年** %d · **无标签** %d  \n\n", tag3Count, tag2Count, noTagCount))

	if !hasLifecycleConfig {
		b.WriteString("> **说明**: 当前 Bucket 未配置 OSS 生命周期规则，Tagged 对象也不会自动删除。\n\n")
	} else {
		b.WriteString("> **说明**: 删除由 OSS Lifecycle 按 **LastModified + Expiration Days** 自动执行（规则加载后约 24h 生效，UTC 0 批次扫描）。\n\n")
	}

	b.WriteString("| 文件时间 | 文件名称 | 文件大小 | 保存周期(tag) | 生命周期规则 | 还有多久删除 |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		name := strings.ReplaceAll(r.name, "|", "\\|")
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
			r.time.Format("2006-01-02 15:04:05"),
			name,
			formatSize(r.size),
			r.tagLabel,
			r.lifecycle,
			r.remaining,
		))
	}

	output := b.String()
	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(output), 0o644); err != nil {
			log.Fatalf("write output: %v", err)
		}
		log.Printf("[INFO] 已写入 %d 条记录到 %s", len(rows), *outPath)
	} else {
		fmt.Print(output)
	}
}
