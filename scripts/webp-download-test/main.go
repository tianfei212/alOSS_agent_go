// 从 OSS 列出图片对象，以 WebP 格式签名 URL 下载到独立目录（默认 100 个）。
//
// 用法:
//
//	go run ./scripts/webp-download-test/ --config config.yaml --count 100
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/derekt/oss-cli/config"
	"github.com/derekt/oss-cli/oss"
)

func main() {
	configPath := flag.String("config", "config.yaml", "config file path")
	count := flag.Int("count", 100, "number of image files to download as webp")
	outDir := flag.String("out", "", "output directory (default: downloads/webp-format-test-<timestamp>)")
	listLimit := flag.Int("list-limit", 1000, "max objects per OSS list page (1-1000)")
	expireSec := flag.Int("expires", 3600, "signed URL expiration in seconds")
	flag.Parse()

	if err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := oss.Init(config.AppConfig.OSS); err != nil {
		log.Fatalf("init oss: %v", err)
	}

	targetDir := *outDir
	if targetDir == "" {
		targetDir = filepath.Join("downloads", "webp-format-test-"+time.Now().Format("20060102_150405"))
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", targetDir, err)
	}

	client := oss.GetInstance()
	prefix := strings.TrimSuffix(config.AppConfig.OSS.BucketPrefix, "/")
	var imageKeys []string
	var continuation string
	scanned := 0

	for len(imageKeys) < *count {
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
		if len(objects) == 0 {
			break
		}
		scanned += len(objects)

		for _, obj := range objects {
			key := obj.Key
			if prefix != "" {
				key = strings.TrimPrefix(key, prefix+"/")
				key = strings.TrimPrefix(key, prefix)
			}
			key = strings.TrimPrefix(key, "/")
			if key == "" || strings.HasSuffix(key, "/") {
				continue
			}
			if !oss.IsImageKey(key) {
				continue
			}
			imageKeys = append(imageKeys, key)
			if len(imageKeys) >= *count {
				break
			}
		}

		if next == "" {
			break
		}
		continuation = next
	}

	if len(imageKeys) == 0 {
		log.Fatalf("no image objects found in bucket (scanned %d objects)", scanned)
	}
	if len(imageKeys) < *count {
		log.Printf("[WARN] only found %d images, downloading all of them (requested %d)", len(imageKeys), *count)
	}

	httpClient := &http.Client{Timeout: 120 * time.Second}
	ok, fail := 0, 0
	reportPath := filepath.Join(targetDir, "download-report.txt")
	report, err := os.Create(reportPath)
	if err != nil {
		log.Fatalf("create report: %v", err)
	}
	defer report.Close()

	fmt.Fprintf(report, "webp download test\n")
	fmt.Fprintf(report, "time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(report, "target: %d images\n\n", len(imageKeys))

	for i, key := range imageKeys {
		signedURL, err := client.GetViewSignedURL(key, 0, 0, "webp", *expireSec)
		if err != nil {
			fail++
			line := fmt.Sprintf("[%d] FAIL sign %s: %v\n", i+1, key, err)
			log.Print(line)
			report.WriteString(line)
			continue
		}
		if !strings.Contains(strings.ToLower(signedURL), "x-oss-process") {
			fail++
			line := fmt.Sprintf("[%d] FAIL sign %s: missing x-oss-process in url\n", i+1, key)
			log.Print(line)
			report.WriteString(line)
			continue
		}

		resp, err := httpClient.Get(signedURL)
		if err != nil {
			fail++
			line := fmt.Sprintf("[%d] FAIL download %s: %v\n", i+1, key, err)
			log.Print(line)
			report.WriteString(line)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			fail++
			line := fmt.Sprintf("[%d] FAIL download %s: status=%d err=%v\n", i+1, key, resp.StatusCode, err)
			log.Print(line)
			report.WriteString(line)
			continue
		}

		base := filepath.Base(key)
		ext := path.Ext(base)
		name := strings.TrimSuffix(base, ext) + ".webp"
		outPath := filepath.Join(targetDir, fmt.Sprintf("%03d_%s", i+1, name))
		if err := os.WriteFile(outPath, body, 0o644); err != nil {
			fail++
			line := fmt.Sprintf("[%d] FAIL write %s: %v\n", i+1, key, err)
			log.Print(line)
			report.WriteString(line)
			continue
		}

		ct := resp.Header.Get("Content-Type")
		ok++
		line := fmt.Sprintf("[%d] OK %s -> %s (%d bytes, Content-Type: %s)\n", i+1, key, outPath, len(body), ct)
		log.Print(strings.TrimSpace(line))
		report.WriteString(line)
	}

	summary := fmt.Sprintf("\nSUMMARY ok=%d fail=%d total=%d dir=%s\n", ok, fail, len(imageKeys), targetDir)
	report.WriteString(summary)
	fmt.Print(summary)

	if fail > 0 {
		os.Exit(1)
	}
}
