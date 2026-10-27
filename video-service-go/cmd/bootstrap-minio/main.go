// Command bootstrap-minio 建桶并设置匿名只读策略。
//
// 为什么不用 compose 里的 minio-init：minio/mc 镜像在部分网络环境拉不下来
// （Docker Hub 返回 pull access denied）。本命令只依赖 video-service 已有的 minio-go。
//
// 用法：
//
//	VIDEO_MINIO_ENDPOINT=127.0.0.1:9000 go run ./cmd/bootstrap-minio
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func main() {
	endpoint := env("VIDEO_MINIO_ENDPOINT", "127.0.0.1:9000")
	accessKey := env("VIDEO_MINIO_ACCESS_KEY", "minioadmin")
	secretKey := env("VIDEO_MINIO_SECRET_KEY", "minioadmin")
	bucket := env("VIDEO_MINIO_BUCKET", "media")
	useSSL := env("VIDEO_MINIO_USE_SSL", "false") == "true"

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		fail("初始化 MinIO 客户端失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		fail("检查桶失败: %v", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			fail("创建桶失败: %v", err)
		}
		fmt.Printf("已创建桶 %s\n", bucket)
	} else {
		fmt.Printf("桶已存在: %s\n", bucket)
	}

	// 只允许匿名读取对象（播放器要直接拉 HLS/封面），不开放写入
	policy := fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::%s/*"]
    }
  ]
}`, bucket)

	if err := client.SetBucketPolicy(ctx, bucket, policy); err != nil {
		fail("设置匿名读策略失败: %v", err)
	}
	fmt.Printf("已设置匿名只读策略: %s\n", bucket)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
