// 为存量 OSS 对象批量设置 retention-years 标签。
//
// 用法: go run ./scripts/set-retention/ --config config.yaml --years 3 [--dry-run] [--skip-existing]
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/derekt/oss-cli/config"
	osscli "github.com/derekt/oss-cli/oss"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	years := flag.Int("years", 3, "retention years to tag on existing objects")
	dryRun := flag.Bool("dry-run", false, "print only, do not tag")
	skipExisting := flag.Bool("skip-existing", false, "skip objects that already have retention-years tag")
	listLimit := flag.Int("list-limit", 1000, "max objects per list page")
	flag.Parse()

	if err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := osscli.ValidateRetentionYears(*years, config.MaxRetentionYears); err != nil {
		log.Fatalf("invalid years: %v", err)
	}
	if err := osscli.Init(config.AppConfig.OSS); err != nil {
		log.Fatalf("init oss: %v", err)
	}

	client := osscli.GetInstance()
	rules, err := client.GetBucketLifecycleRules()
	if err != nil {
		log.Fatalf("get lifecycle: %v", err)
	}
	if !osscli.HasRetentionLifecycleRule(rules, *years) {
		log.Fatalf("Bucket 缺少 retention-years-%d 生命周期规则，请先运行 scripts/sync-lifecycle-rules.sh", *years)
	}

	bucketPrefix := strings.TrimSuffix(config.AppConfig.OSS.BucketPrefix, "/")
	var total, updated, skipped, failed int
	var continuation string

	for {
		pageLimit := *listLimit
		if pageLimit < 1 || pageLimit > 1000 {
			pageLimit = 1000
		}
		objects, next, err := client.ListFilesPage("", pageLimit, continuation)
		if err != nil {
			log.Fatalf("list files: %v", err)
		}
		for _, obj := range objects {
			if strings.HasSuffix(obj.Key, "/") {
				continue
			}
			publicKey := stripPrefix(obj.Key, bucketPrefix)
			if publicKey == "" {
				continue
			}
			total++

			if *skipExisting {
				if y, ok, tagErr := client.GetObjectRetentionTag(publicKey); tagErr == nil && ok {
					if y == *years {
						skipped++
						continue
					}
				}
			}

			if *dryRun {
				fmt.Printf("[dry-run] would tag %s -> retention-years=%d\n", publicKey, *years)
				updated++
				continue
			}

			if err := client.PutObjectRetentionTag(publicKey, *years); err != nil {
				log.Printf("[ERROR] tag failed %s: %v", publicKey, err)
				failed++
				continue
			}
			updated++
			if updated%100 == 0 {
				log.Printf("[INFO] 已处理 %d 个对象...", updated)
			}
		}
		if next == "" {
			break
		}
		continuation = next
	}

	fmt.Printf("\n完成: total=%d updated=%d skipped=%d failed=%d dry_run=%v\n",
		total, updated, skipped, failed, *dryRun)
	if failed > 0 {
		log.Fatalf("部分对象打标签失败")
	}
}

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
