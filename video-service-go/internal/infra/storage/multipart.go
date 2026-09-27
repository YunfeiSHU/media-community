package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/minio/minio-go/v7"
)

// 本文件封装 S3 多分片上传（multipart upload）原语，用于大文件分片上传与断点续传。
//
// 为什么断点续传可以「只靠 uploadId + 对象存储」成立：
//   - 分片是**对象存储侧**的状态，服务进程重启、扩容、换实例都不会丢；
//   - 因而服务端不需要缓存「第几片传过了」，随时 ListParts 就能得到权威答案。
//
// 服务端唯一必须持久化的东西是 `uploadId ↔ videoId` 的映射（见 media_upload_session 表），
// 因为 uploadId 由对象存储生成，客户端刷新页面后就拿不到了。

// ErrUploadNotFound 表示 uploadId 在对象存储侧已不存在。
//
// 单独抽出一个哨兵错误，是因为调用方必须把它和「对象存储暂时不可用」区分开：
// 前者意味着会话真的废了（可以安全地丢弃重建），后者只是网络抖动
// —— 如果错把网络抖动当失效，用户已经传好的几 GB 分片会被白白清掉。
var ErrUploadNotFound = errors.New("分片上传会话在对象存储侧不存在")

// Part 是分片元信息。
//
// 刻意不对外暴露 minio.ObjectPart：service 层不应依赖对象存储 SDK 的类型，
// 否则将来换成 S3 / OSS / 云厂商实现时，业务代码要跟着一起改。
type Part struct {
	PartNumber int
	Size       int64
	ETag       string
}

// NewMultipartUpload 初始化一次分片上传，返回对象存储侧的 uploadId。
func (s *Storage) NewMultipartUpload(ctx context.Context, key, contentType string) (string, error) {
	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}
	uploadID, err := s.core.NewMultipartUpload(ctx, s.bucket, key, opts)
	if err != nil {
		return "", fmt.Errorf("初始化分片上传 %s 失败: %w", key, err)
	}
	return uploadID, nil
}

// PutPart 上传单个分片。
//
// partNumber 从 1 开始；除最后一片外，每片不得小于 5MiB（S3 协议约束，
// 由调用方在 service 层校验，这里只负责传输）。
func (s *Storage) PutPart(
	ctx context.Context, key, uploadID string, partNumber int, r io.Reader, size int64,
) (Part, error) {
	part, err := s.core.PutObjectPart(ctx, s.bucket, key, uploadID, partNumber, r, size,
		minio.PutObjectPartOptions{})
	if err != nil {
		return Part{}, fmt.Errorf("上传分片 %s#%d 失败: %w", key, partNumber, err)
	}
	return toPart(part), nil
}

// ListParts 拉取某个上传会话已上传的全部分片（自动翻页，按 partNumber 升序）。
//
// 这是「断点续传」的权威数据源：返回哪些就是哪些，服务端不需要自己记账。
func (s *Storage) ListParts(ctx context.Context, key, uploadID string) ([]Part, error) {
	// S3 单次最多返回 1000 个分片，分页拉到不截断为止。
	const pageSize = 1000

	var (
		marker int
		all    []Part
	)
	for {
		result, err := s.core.ListObjectParts(ctx, s.bucket, key, uploadID, marker, pageSize)
		if err != nil {
			if isUploadNotFound(err) {
				return nil, fmt.Errorf("查询分片列表 %s: %w", key, ErrUploadNotFound)
			}
			return nil, fmt.Errorf("查询分片列表 %s 失败: %w", key, err)
		}
		for _, p := range result.ObjectParts {
			all = append(all, toPart(p))
		}
		// 没有更多分片，或服务端返回空页（防御：避免 marker 不前进导致死循环）
		if !result.IsTruncated || len(result.ObjectParts) == 0 {
			break
		}
		marker = result.NextPartNumberMarker
	}

	sort.Slice(all, func(i, j int) bool { return all[i].PartNumber < all[j].PartNumber })
	return all, nil
}

// CompleteMultipart 按顺序合并分片成完整对象。
//
// parts 的顺序即最终对象的字节顺序，必须由调用方按 partNumber 升序传入。
func (s *Storage) CompleteMultipart(ctx context.Context, key, uploadID string, parts []Part) error {
	if len(parts) == 0 {
		return fmt.Errorf("合并分片 %s 失败: 分片列表为空", key)
	}
	complete := make([]minio.CompletePart, 0, len(parts))
	for _, p := range parts {
		complete = append(complete, minio.CompletePart{PartNumber: p.PartNumber, ETag: p.ETag})
	}
	if _, err := s.core.CompleteMultipartUpload(ctx, s.bucket, key, uploadID, complete,
		minio.PutObjectOptions{}); err != nil {
		return fmt.Errorf("合并分片 %s 失败: %w", key, err)
	}
	return nil
}

// AbortMultipart 取消分片上传，并让对象存储回收已上传的分片。
//
// 必须调用：不 abort 的话已上传的分片会一直占着存储空间，且永远不会出现在任何列举结果里。
func (s *Storage) AbortMultipart(ctx context.Context, key, uploadID string) error {
	if err := s.core.AbortMultipartUpload(ctx, s.bucket, key, uploadID); err != nil {
		return fmt.Errorf("取消分片上传 %s 失败: %w", key, err)
	}
	return nil
}

// StatSize 返回对象大小（用于校验合并结果是否与声明的大小一致）。
func (s *Storage) StatSize(ctx context.Context, key string) (int64, error) {
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, fmt.Errorf("读取对象信息 %s 失败: %w", key, err)
	}
	return info.Size, nil
}

// toPart 把 SDK 类型投影成本包自有类型。
func toPart(p minio.ObjectPart) Part {
	return Part{PartNumber: p.PartNumber, Size: p.Size, ETag: p.ETag}
}

// isUploadNotFound 判断对象存储返回的是否为 NoSuchUpload。
func isUploadNotFound(err error) bool {
	var resp minio.ErrorResponse
	return errors.As(err, &resp) && resp.Code == "NoSuchUpload"
}
