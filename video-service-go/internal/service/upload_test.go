package service

import (
	"errors"
	"testing"
	"time"

	"github.com/example/video-service-go/internal/infra/storage"
	"github.com/example/video-service-go/internal/model"
	"github.com/example/video-service-go/pkg/errs"
)

const (
	mib          = 1 << 20
	fiveMiB      = 5 * mib
	twentyMiB    = 20 * mib
	hundredGiB   = 100 * 1024 * mib
	defaultParts = 10000
)

// codeOf 取出业务错误码，非业务错误返回 -1。
func codeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		t.Fatal("期望返回错误，实际为 nil")
	}
	var biz *errs.BizError
	if !errors.As(err, &biz) {
		t.Fatalf("期望 *errs.BizError，实际为 %T: %v", err, err)
	}
	return biz.Code.Code
}

func TestChoosePartSize(t *testing.T) {
	cases := []struct {
		name     string
		fileSize int64
		client   int64
		want     int64
	}{
		{"小文件用最小值（单片）", 10 * mib, 0, fiveMiB},
		{"客户端指定更大分片时以其为准", 10 * mib, 8 * mib, 8 * mib},
		{"客户端指定小于下限时忽略（下限不可突破）", 10 * mib, mib, fiveMiB},
		{
			// 100GiB / 5MiB = 20480 片，超过 10000 上限 → 逐次翻倍到 20MiB（5120 片）
			"文件很大时自动放大分片以控制片数",
			hundredGiB, 0, 20 * mib,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := choosePartSize(tc.fileSize, tc.client, fiveMiB, defaultParts)
			if got != tc.want {
				t.Fatalf("choosePartSize(%d, %d) = %d, 期望 %d", tc.fileSize, tc.client, got, tc.want)
			}
			if parts := totalPartsOf(tc.fileSize, got); parts > defaultParts {
				t.Fatalf("分片数 %d 超过上限 %d", parts, defaultParts)
			}
		})
	}
}

func TestTotalPartsOf(t *testing.T) {
	cases := []struct {
		fileSize int64
		partSize int64
		want     int
	}{
		{0, fiveMiB, 0},
		{fiveMiB, fiveMiB, 1},
		// 多出 1 字节也必须多开一片，否则最后一片会被截断
		{fiveMiB + 1, fiveMiB, 2},
		{twentyMiB, 8 * mib, 3},
	}

	for _, tc := range cases {
		if got := totalPartsOf(tc.fileSize, tc.partSize); got != tc.want {
			t.Fatalf("totalPartsOf(%d, %d) = %d, 期望 %d", tc.fileSize, tc.partSize, got, tc.want)
		}
	}
}

func TestSessionReusable(t *testing.T) {
	base := model.UploadSession{
		ObjectKey:  "video/original/1/source.mp4",
		Ext:        "mp4",
		FileSize:   20 * mib,
		PartSize:   8 * mib,
		TotalParts: 3,
		Status:     model.UploadStatusUploading,
		CreatedAt:  time.Now(),
	}

	cases := []struct {
		name     string
		mutate   func(*model.UploadSession)
		reusable bool
	}{
		{"参数完全一致且在有效期内", func(*model.UploadSession) {}, true},
		{"扩展名不同（续传会拼错文件）", func(s *model.UploadSession) { s.Ext = "mov" }, false},
		{"总大小不同", func(s *model.UploadSession) { s.FileSize = 21 * mib }, false},
		{"分片大小不同（偏移量全错）", func(s *model.UploadSession) { s.PartSize = 5 * mib }, false},
		{"分片数不同", func(s *model.UploadSession) { s.TotalParts = 2 }, false},
		{"对象 key 不同", func(s *model.UploadSession) { s.ObjectKey = "video/original/2/source.mp4" }, false},
		{"会话已完成", func(s *model.UploadSession) { s.Status = model.UploadStatusCompleted }, false},
		{"会话已取消", func(s *model.UploadSession) { s.Status = model.UploadStatusAborted }, false},
		{"超过有效期", func(s *model.UploadSession) { s.CreatedAt = time.Now().Add(-uploadSessionTTL - time.Minute) }, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := base
			tc.mutate(&session)
			got := sessionReusable(session, base.ObjectKey, base.Ext, base.FileSize, base.PartSize, base.TotalParts)
			if got != tc.reusable {
				t.Fatalf("sessionReusable = %v, 期望 %v", got, tc.reusable)
			}
		})
	}
}

func TestCheckPartsComplete(t *testing.T) {
	session := &model.UploadSession{TotalParts: 3, FileSize: 20 * mib, PartSize: 8 * mib}

	cases := []struct {
		name  string
		parts []storage.Part
		want  int // 0 表示期望通过
	}{
		{
			name: "分片齐全",
			parts: []storage.Part{
				{PartNumber: 1, Size: 8 * mib},
				{PartNumber: 2, Size: 8 * mib},
				{PartNumber: 3, Size: 4 * mib},
			},
		},
		{
			name: "少一片",
			parts: []storage.Part{
				{PartNumber: 1, Size: 8 * mib},
				{PartNumber: 2, Size: 8 * mib},
			},
			want: 60006,
		},
		{
			// 数量够但序号重复：并发写同一个 partNumber 时会走到这里，
			// 合并出来的对象会缺字节，必须在合并前挡住
			name: "序号重复",
			parts: []storage.Part{
				{PartNumber: 1, Size: 8 * mib},
				{PartNumber: 1, Size: 8 * mib},
				{PartNumber: 3, Size: 4 * mib},
			},
			want: 60006,
		},
		{
			name: "序号越界",
			parts: []storage.Part{
				{PartNumber: 1, Size: 8 * mib},
				{PartNumber: 2, Size: 8 * mib},
				{PartNumber: 9, Size: 4 * mib},
			},
			want: 60005,
		},
		{
			name: "总大小与声明不符",
			parts: []storage.Part{
				{PartNumber: 1, Size: 8 * mib},
				{PartNumber: 2, Size: 8 * mib},
				{PartNumber: 3, Size: 1 * mib},
			},
			want: 60006,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkPartsComplete(session, tc.parts)
			if tc.want == 0 {
				if err != nil {
					t.Fatalf("期望通过，实际报错: %v", err)
				}
				return
			}
			if got := codeOf(t, err); got != tc.want {
				t.Fatalf("错误码 = %d, 期望 %d", got, tc.want)
			}
		})
	}

	t.Run("会话缺少分片计划", func(t *testing.T) {
		err := checkPartsComplete(&model.UploadSession{TotalParts: 0}, nil)
		if got := codeOf(t, err); got != 60005 {
			t.Fatalf("错误码 = %d, 期望 60005", got)
		}
	})
}

func TestBuildSnapshot(t *testing.T) {
	session := &model.UploadSession{
		VideoID:    7,
		UploadID:   "uid-1",
		ObjectKey:  "video/original/7/source.mp4",
		Ext:        "mp4",
		FileSize:   20 * mib,
		PartSize:   8 * mib,
		TotalParts: 3,
	}

	// 未传任何分片：Completed 必须为 false，否则客户端会误以为可以直接合并
	empty := buildSnapshot(session, nil)
	if empty.Completed || empty.UploadedCount != 0 || empty.UploadedBytes != 0 {
		t.Fatalf("空进度快照不合法: %+v", empty)
	}

	partial := buildSnapshot(session, []storage.Part{
		{PartNumber: 1, Size: 8 * mib, ETag: "e1"},
	})
	if partial.Completed || partial.UploadedCount != 1 || partial.UploadedBytes != 8*mib {
		t.Fatalf("部分进度快照不合法: %+v", partial)
	}

	full := buildSnapshot(session, []storage.Part{
		{PartNumber: 1, Size: 8 * mib, ETag: "e1"},
		{PartNumber: 2, Size: 8 * mib, ETag: "e2"},
		{PartNumber: 3, Size: 4 * mib, ETag: "e3"},
	})
	if !full.Completed || full.UploadedCount != 3 || full.UploadedBytes != 20*mib {
		t.Fatalf("完整进度快照不合法: %+v", full)
	}
	if full.TotalParts != 3 || full.PartSize != 8*mib || full.FileSize != 20*mib {
		t.Fatalf("快照丢失了会话参数: %+v", full)
	}
}
