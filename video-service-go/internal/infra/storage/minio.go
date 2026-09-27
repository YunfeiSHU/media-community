// Package storage 封装对象存储（MinIO）。
//
// 约定：视频文件一律不进数据库，数据库只保存对象 key（如 video/original/1001/source.mp4）。
package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/example/video-service-go/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage 是对象存储客户端封装。
type Storage struct {
	client     *minio.Client
	bucket     string
	publicBase string
}

// New 构造对象存储客户端。endpoint 不含 scheme（如 127.0.0.1:9000）。
func New(cfg config.MinIO) (*Storage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 MinIO 客户端失败: %w", err)
	}
	return &Storage{
		client:     client,
		bucket:     cfg.Bucket,
		publicBase: strings.TrimRight(cfg.PublicEndpoint, "/"),
	}, nil
}

// Bucket 返回桶名。
func (s *Storage) Bucket() string { return s.bucket }

// EnsureBucket 确保桶存在（幂等）。
func (s *Storage) EnsureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("检查桶 %s 失败: %w", s.bucket, err)
	}
	if ok {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		// 并发创建时可能已被其他实例创建，再查一次
		if ok2, err2 := s.client.BucketExists(ctx, s.bucket); err2 == nil && ok2 {
			return nil
		}
		return fmt.Errorf("创建桶 %s 失败: %w", s.bucket, err)
	}
	return nil
}

// Put 流式写入对象（不把文件整体读进内存）。
func (s *Storage) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}
	if _, err := s.client.PutObject(ctx, s.bucket, key, r, size, opts); err != nil {
		return fmt.Errorf("上传对象 %s 失败: %w", key, err)
	}
	return nil
}

// PutBytes 写入内存中的对象（用于 m3u8 这类小文本）。
func (s *Storage) PutBytes(ctx context.Context, key string, data []byte, contentType string) error {
	return s.Put(ctx, key, bytes.NewReader(data), int64(len(data)), contentType)
}

// GetBytes 读取对象内容（用于 real 转码器把原片下载到本地）。
func (s *Storage) GetBytes(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("读取对象 %s 失败: %w", key, err)
	}
	defer func() { _ = obj.Close() }()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("读取对象 %s 内容失败: %w", key, err)
	}
	return data, nil
}

// Download 把对象下载到本地文件（大文件走流式，不进内存）。
func (s *Storage) Download(ctx context.Context, key, destPath string) error {
	if err := s.client.FGetObject(ctx, s.bucket, key, destPath, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("下载对象 %s 到 %s 失败: %w", key, destPath, err)
	}
	return nil
}

// PutFile 上传本地文件为对象。
func (s *Storage) PutFile(ctx context.Context, key, localPath, contentType string) error {
	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}
	if _, err := s.client.FPutObject(ctx, s.bucket, key, localPath, opts); err != nil {
		return fmt.Errorf("上传本地文件 %s 为对象 %s 失败: %w", localPath, key, err)
	}
	return nil
}

// RemovePrefix 删除某个前缀下的全部对象（软删除视频时清理产物）。
func (s *Storage) RemovePrefix(ctx context.Context, prefix string) error {
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		}) {
			if obj.Err != nil {
				continue
			}
			objectsCh <- obj
		}
	}()

	var firstErr error
	for rmErr := range s.client.RemoveObjects(ctx, s.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if rmErr.Err != nil && firstErr == nil {
			firstErr = rmErr.Err
		}
	}
	if firstErr != nil {
		return fmt.Errorf("删除前缀 %s 失败: %w", prefix, firstErr)
	}
	return nil
}

// URL 拼装对外可访问的完整地址（供前端播放）。
func (s *Storage) URL(key string) string {
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", s.publicBase, s.bucket, strings.TrimLeft(key, "/"))
}

// Health 用于 /readyz 探针。
func (s *Storage) Health(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("桶 %s 不存在", s.bucket)
	}
	return nil
}
