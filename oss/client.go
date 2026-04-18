package oss

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/derekt/oss-cli/config"
	"github.com/google/uuid"
)

// ProgressReader 包装 io.Reader，在读取数据的过程中每隔一段时间打印一次已读取的字节数
type ProgressReader struct {
	reader      io.Reader
	objectKey   string
	totalSize   int64
	uploadStart time.Time
	bytesRead   int64
	stopChan    chan struct{}
}

type progressListenerForSDK struct {
	objectKey   string
	fileSize    int64
	uploadStart time.Time
}

func (p *progressListenerForSDK) ProgressChanged(event *oss.ProgressEvent) {
	uploaded := event.ConsumedBytes
	fileSize := p.fileSize
	if fileSize == 0 {
		return
	}
	percent := float64(uploaded) / float64(fileSize) * 100.0
	elapsed := time.Since(p.uploadStart).Seconds()
	speed := float64(uploaded) / 1024.0 / 1024.0 / elapsed
	log.Printf("[INFO] 上传进度 [%s] %.2f%% (%d / %d 字节), 速度: %.2f MB/s",
		p.objectKey, percent, uploaded, fileSize, speed)
}

func (p *ProgressReader) Read(b []byte) (int, error) {
	n, err := p.reader.Read(b)
	if n > 0 {
		atomic.AddInt64(&p.bytesRead, int64(n))
	}
	return n, err
}

func (p *ProgressReader) startLogging() {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				elapsed := time.Since(p.uploadStart).Seconds()
				bytes := atomic.LoadInt64(&p.bytesRead)
				speed := float64(bytes) / 1024.0 / 1024.0 / elapsed
				if elapsed > 0 {
					log.Printf("[INFO] 上传进度 [%s] 已上传: %d 字节 (%.2f MB), 当前速度: %.2f MB/s", p.objectKey, bytes, float64(bytes)/1024.0/1024.0, speed)
				}
			case <-p.stopChan:
				return
			}
		}
	}()
}

func (p *ProgressReader) stop() {
	close(p.stopChan)
}

// Client 封装了阿里云 OSS 客户端对象
type Client struct {
	Bucket *oss.Bucket
	Prefix string
}

var instance *Client

// Init 初始化阿里云 OSS 客户端，连接到指定的 Bucket
func Init(cfg config.OSSConfig) error {
	log.Printf("[INFO] 初始化 OSS 客户端，Endpoint: %s, Bucket: %s", cfg.Endpoint, cfg.BucketName)
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		log.Printf("[ERROR] 创建 OSS 客户端失败: %v\n", err)
		return fmt.Errorf("创建 OSS 客户端失败: %w", err)
	}

	bucket, err := client.Bucket(cfg.BucketName)
	if err != nil {
		log.Printf("[ERROR] 获取 OSS Bucket 失败: %v\n", err)
		return fmt.Errorf("获取 OSS Bucket 失败: %w", err)
	}

	instance = &Client{
		Bucket: bucket,
		Prefix: cfg.BucketPrefix,
	}

	log.Println("[INFO] OSS 客户端初始化成功")
	return nil
}

// GetInstance 获取初始化的 OSS 客户端单例
func GetInstance() *Client {
	return instance
}

// resolveKey 处理文件前缀，拼接配置中的 BucketPrefix
func (c *Client) resolveKey(key string) string {
	if c.Prefix != "" {
		return path.Join(c.Prefix, key)
	}
	return key
}

// UploadFile 执行本地文件的多部分并发上传至 OSS
func (c *Client) UploadFile(localFilePath string, objectKey string) error {
	if objectKey == "" {
		objectKey = "file-" + uuid.New().String() + path.Ext(localFilePath)
	}

	finalKey := c.resolveKey(objectKey)
	log.Printf("[INFO] 开始上传本地文件: %s 到 OSS: %s", localFilePath, finalKey)

	fileInfo, err := os.Stat(localFilePath)
	if err != nil {
		log.Printf("[WARN] 无法获取本地文件大小: %v，将使用默认方式上传", err)
		err := c.Bucket.UploadFile(finalKey, localFilePath, 100*1024, oss.Routines(3))
		if err != nil {
			log.Printf("[ERROR] 本地文件上传失败 (localPath: %s, key: %s): %v\n", localFilePath, finalKey, err)
			return fmt.Errorf("本地文件上传失败: %w", err)
		}
		log.Printf("[INFO] 文件上传成功: %s", finalKey)
		return nil
	}

	fileSize := fileInfo.Size()
	log.Printf("[INFO] 文件总大小: %d 字节 (%.2f MB)，开始分片上传", fileSize, float64(fileSize)/1024.0/1024.0)

	progressReader := &ProgressReader{
		objectKey:   finalKey,
		totalSize:   fileSize,
		uploadStart: time.Now(),
		stopChan:   make(chan struct{}),
	}
	progressReader.startLogging()

	pl := &progressListenerForSDK{
		objectKey:   finalKey,
		fileSize:    fileSize,
		uploadStart: progressReader.uploadStart,
	}

	err = c.Bucket.UploadFile(finalKey, localFilePath, 100*1024,
		oss.Routines(3),
		oss.Progress(pl),
	)

	progressReader.stop()

	if err != nil {
		log.Printf("[ERROR] 本地文件上传失败 (localPath: %s, key: %s): %v\n", localFilePath, finalKey, err)
		return fmt.Errorf("本地文件上传失败: %w", err)
	}

	log.Printf("[INFO] 文件上传成功: %s", finalKey)
	return nil
}

// UploadStream 执行流式上传，主要用于处理 HTTP 请求的文件流，并实时打印进度
func (c *Client) UploadStream(reader io.Reader, objectKey string) error {
	finalKey := c.resolveKey(objectKey)
	log.Printf("[INFO] 开始流式上传到 OSS: %s", finalKey)

	progressReader := &ProgressReader{
		reader:      reader,
		objectKey:   finalKey,
		uploadStart: time.Now(),
		stopChan:   make(chan struct{}),
	}
	progressReader.startLogging()

	err := c.Bucket.PutObject(finalKey, progressReader)
	progressReader.stop()

	if err != nil {
		log.Printf("[ERROR] 流式上传失败 (key: %s): %v\n", finalKey, err)
		return fmt.Errorf("流式上传失败: %w", err)
	}

	bytes := atomic.LoadInt64(&progressReader.bytesRead)
	elapsed := time.Since(progressReader.uploadStart).Seconds()
	log.Printf("[INFO] 流式上传成功: %s，已上传 %d 字节，耗时 %.2f 秒，平均速度: %.2f MB/s",
		finalKey, bytes, elapsed, float64(bytes)/1024.0/1024.0/elapsed)
	return nil
}

// DeleteFile 从 OSS 删除指定的文件对象
func (c *Client) DeleteFile(objectKey string) error {
	finalKey := c.resolveKey(objectKey)
	log.Printf("[INFO] 正在删除 OSS 文件: %s", finalKey)

	err := c.Bucket.DeleteObject(finalKey)
	if err != nil {
		log.Printf("[ERROR] 删除文件失败 (key: %s): %v\n", finalKey, err)
		return fmt.Errorf("删除文件失败: %w", err)
	}

	log.Printf("[INFO] 删除文件成功: %s", finalKey)
	return nil
}

// ListFiles 列出 OSS 中的文件列表
func (c *Client) ListFiles(prefix string, maxKeys int) ([]oss.ObjectProperties, error) {
	finalPrefix := c.resolveKey(prefix)
	log.Printf("[INFO] 列出 OSS 文件，前缀: %s，最大数量: %d", finalPrefix, maxKeys)

	lsRes, err := c.Bucket.ListObjectsV2(oss.Prefix(finalPrefix), oss.MaxKeys(maxKeys))
	if err != nil {
		log.Printf("[ERROR] 列出文件失败 (prefix: %s): %v\n", finalPrefix, err)
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}

	log.Printf("[INFO] 成功列出 %d 个文件", len(lsRes.Objects))
	return lsRes.Objects, nil
}

// GetSignedURL 生成带有有效期的临时下载/播放链接
func (c *Client) GetSignedURL(objectKey string, expiredInSec int) (string, error) {
	finalKey := c.resolveKey(objectKey)
	log.Printf("[INFO] 生成文件签名 URL，文件: %s，有效期: %d 秒", finalKey, expiredInSec)

	signedURL, err := c.Bucket.SignURL(finalKey, oss.HTTPGet, int64(expiredInSec))
	if err != nil {
		log.Printf("[ERROR] 生成签名 URL 失败 (key: %s): %v\n", finalKey, err)
		return "", fmt.Errorf("生成签名 URL 失败: %w", err)
	}

	log.Printf("[INFO] 签名 URL 生成成功")
	return signedURL, nil
}

// GetFileInfo 获取 OSS 中对象的文件元数据
func (c *Client) GetFileInfo(objectKey string) (oss.ObjectProperties, error) {
	finalKey := c.resolveKey(objectKey)
	log.Printf("[INFO] 获取文件信息: %s", finalKey)

	props, err := c.Bucket.GetObjectMeta(finalKey)
	if err != nil {
		log.Printf("[ERROR] 获取文件信息失败 (key: %s): %v\n", finalKey, err)
		return oss.ObjectProperties{}, fmt.Errorf("获取文件信息失败: %w", err)
	}

	size := int64(0)
	lsRes, err := c.Bucket.ListObjectsV2(oss.Prefix(finalKey), oss.MaxKeys(1))
	if err == nil && len(lsRes.Objects) > 0 && lsRes.Objects[0].Key == finalKey {
		log.Printf("[INFO] 成功获取文件详情，文件大小: %d 字节", lsRes.Objects[0].Size)
		return lsRes.Objects[0], nil
	}

	_ = props
	log.Printf("[INFO] 返回基础文件元数据，无法确定确切大小")
	return oss.ObjectProperties{Key: finalKey, Size: size}, nil
}
