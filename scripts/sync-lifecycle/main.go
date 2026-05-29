// 同步 OSS Bucket 保存周期 Lifecycle 规则（merge 写入，不覆盖非 retention 规则）。
//
// 用法: go run ./scripts/sync-lifecycle/ --config config.yaml [--dry-run]
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/derekt/oss-cli/config"
	osscli "github.com/derekt/oss-cli/oss"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	dryRun := flag.Bool("dry-run", false, "print rules only, do not apply")
	flag.Parse()

	if err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := osscli.Init(config.AppConfig.OSS); err != nil {
		log.Fatalf("init oss: %v", err)
	}

	yearsList := config.AllowedRetentionYears()
	daysList := config.AllowedRetentionDays()
	client := osscli.GetInstance()
	prefix := config.AppConfig.OSS.BucketPrefix

	fmt.Printf("将同步 %d 条按年 + %d 条按天 retention Lifecycle 规则，前缀: %q\n", len(yearsList), len(daysList), prefix)
	for _, y := range yearsList {
		rule := osscli.BuildRetentionLifecycleRule(prefix, y)
		days := 0
		if rule.Expiration != nil {
			days = rule.Expiration.Days
		}
		fmt.Printf("  - %s: tag retention-years=%d, Expiration Days=%d\n", rule.ID, y, days)
	}
	for _, d := range daysList {
		rule := osscli.BuildRetentionDaysLifecycleRule(prefix, d)
		fmt.Printf("  - %s: tag retention-days=%d, Expiration Days=%d\n", rule.ID, d, d)
	}

	if *dryRun {
		fmt.Println("dry-run 模式，未写入 OSS")
		return
	}

	if err := client.SyncRetentionLifecycleRules(prefix, yearsList, daysList); err != nil {
		log.Fatalf("sync lifecycle: %v", err)
	}

	rules, err := client.GetBucketLifecycleRules()
	if err != nil {
		log.Fatalf("verify lifecycle: %v", err)
	}
	count := 0
	for _, r := range rules {
		if strings.HasPrefix(r.ID, "retention-years-") || strings.HasPrefix(r.ID, "retention-days-") {
			count++
		}
	}
	fmt.Printf("同步完成，当前 Bucket 共 %d 条 retention 规则\n", count)
	os.Exit(0)
}
