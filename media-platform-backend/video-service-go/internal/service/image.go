package service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/video-service-go/internal/infra/storage"
	"github.com/example/video-service-go/pkg/errs"
	"github.com/google/uuid"
)

// 图片上传限制。
const (
	// MaxImageBytes 单文件上限 5MB（与前端预检、Nginx client_max_body_size 对齐）
	MaxImageBytes = 5 << 20

	maxCoverURLLen = 512
)

// 允许的图片使用场景。新增场景要同步前端 scene 枚举。
var allowedImageScenes = map[string]bool{
	"video_cover":   true,
	"avatar":        true,
	"product_cover": true,
	"live_cover":    true,
}

// 允许的图片类型：按**文件头**判断，不看扩展名。
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

// ImageResult 是图片上传的响应。
type ImageResult struct {
	URL       string            `json:"url"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	SizeBytes int64             `json:"sizeBytes"`
	Variants  map[string]string `json:"variants"`
}

// ImageService 处理图片校验、压缩与对象存储落盘。
//
// 只依赖标准库：本仓库的 Go module cache 在部分环境下不可写，
// 因此没有引入 golang.org/x/image；WebP 只做存储与文件头校验，不生成缩略图。
type ImageService struct {
	store *storage.Storage
}

// NewImageService 构造图片服务。
func NewImageService(store *storage.Storage) *ImageService {
	return &ImageService{store: store}
}

// SaveImage 校验并保存图片，返回原图与两档缩略图地址。
func (s *ImageService) SaveImage(ctx context.Context, uid, scene string, data []byte) (*ImageResult, error) {
	if !allowedImageScenes[scene] {
		return nil, errs.CodeInvalidParam.New("scene 必须是 video_cover / avatar / product_cover / live_cover")
	}
	if len(data) == 0 {
		return nil, errs.CodeInvalidParam.New("file 不能为空")
	}
	if len(data) > MaxImageBytes {
		return nil, errs.CodeFileTooLarge.Newf("图片不能超过 %d MB", MaxImageBytes>>20)
	}

	contentType := http.DetectContentType(data)
	ext, ok := allowedImageTypes[contentType]
	if !ok {
		return nil, errs.CodeUnsupportedFileType.New("只支持 jpg / png / webp")
	}

	now := time.Now()
	key := fmt.Sprintf("image/%s/%s/%s%s", scene, now.Format("2006/01"), uuid.NewString(), ext)

	width, height, thumbs, err := buildImageVariants(data, ext)
	if err != nil {
		return nil, err
	}

	if err := s.store.PutBytes(ctx, key, data, contentType); err != nil {
		return nil, errs.CodeInternal.New("保存图片失败")
	}

	variants := map[string]string{}
	// 缩略图为空表示该格式不支持缩放（WebP），前端直接回落到原图
	for name, thumb := range thumbs {
		thumbKey := strings.TrimSuffix(key, path.Ext(key)) + "_" + name + ext
		if err := s.store.PutBytes(ctx, thumbKey, thumb, contentType); err != nil {
			return nil, errs.CodeInternal.New("保存缩略图失败")
		}
		variants[name] = s.store.URL(thumbKey)
	}

	url := s.store.URL(key)
	if len(variants) == 0 {
		variants["s"] = url
		variants["m"] = url
	}

	return &ImageResult{
		URL:       url,
		Width:     width,
		Height:    height,
		SizeBytes: int64(len(data)),
		Variants:  variants,
	}, nil
}

// buildImageVariants 解析尺寸并生成 320w / 640w 两档缩略图。
//
// 返回的 thumbs 可能为空（WebP 或原图已小于目标宽度），调用方据此回落。
func buildImageVariants(data []byte, ext string) (int, int, map[string][]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		// WebP：标准库不支持解码，只保留原图
		if ext == ".webp" {
			return 0, 0, nil, nil
		}
		return 0, 0, nil, errs.CodeUnsupportedFileType.New("图片内容无法解析")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		if ext == ".webp" {
			return cfg.Width, cfg.Height, nil, nil
		}
		return 0, 0, nil, errs.CodeUnsupportedFileType.New("图片内容无法解析")
	}

	thumbs := make(map[string][]byte, 2)
	for _, target := range []struct {
		name string
		wide int
	}{{"s", 320}, {"m", 640}} {
		if cfg.Width <= target.wide {
			continue
		}
		resized := downscale(img, target.wide)
		encoded, err := encodeImage(resized, ext)
		if err != nil {
			return 0, 0, nil, errs.CodeInternal.New("生成缩略图失败")
		}
		thumbs[target.name] = encoded
	}
	return cfg.Width, cfg.Height, thumbs, nil
}

// downscale 用盒式采样把图片缩到指定宽度（只缩不放）。
//
// 选盒式而不是双线性：缩小时它对每个目标像素做区域平均，边缘更干净，
// 且不需要额外依赖；对封面这种 1280→320 的 4 倍缩小效果足够。
func downscale(src image.Image, targetWidth int) image.Image {
	b := src.Bounds()
	if b.Dx() <= targetWidth || b.Dx() == 0 {
		return src
	}
	ratio := float64(b.Dx()) / float64(targetWidth)
	targetHeight := int(math.Round(float64(b.Dy()) / ratio))
	if targetHeight < 1 {
		targetHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	for y := 0; y < targetHeight; y++ {
		y0 := b.Min.Y + int(float64(y)*ratio)
		y1 := b.Min.Y + int(float64(y+1)*ratio)
		if y1 <= y0 {
			y1 = y0 + 1
		}
		if y1 > b.Max.Y {
			y1 = b.Max.Y
		}
		for x := 0; x < targetWidth; x++ {
			x0 := b.Min.X + int(float64(x)*ratio)
			x1 := b.Min.X + int(float64(x+1)*ratio)
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if x1 > b.Max.X {
				x1 = b.Max.X
			}

			var r, g, bl, a, n uint32
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					cr, cg, cb, ca := src.At(xx, yy).RGBA()
					r += cr >> 8
					g += cg >> 8
					bl += cb >> 8
					a += ca >> 8
					n++
				}
			}
			if n == 0 {
				continue
			}
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r / n),
				G: uint8(g / n),
				B: uint8(bl / n),
				A: uint8(a / n),
			})
		}
	}
	return dst
}

func encodeImage(img image.Image, ext string) ([]byte, error) {
	var buf bytes.Buffer
	var err error
	if ext == ".png" {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82})
	}
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NormalizeCoverURL 校验并归一化封面地址。
//
// 只接受 http(s) 绝对地址或以 / 开头的站内相对路径：
// 前端会把返回值直接塞进 <img src>，放行 javascript: / data: 会变成 XSS 入口。
func NormalizeCoverURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if utf8.RuneCountInString(trimmed) > maxCoverURLLen {
		return "", errs.CodeInvalidParam.Newf("coverUrl 长度不能超过 %d", maxCoverURLLen)
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(trimmed, "/") {
		// 防止 //evil.com 这类协议相对地址被当成站内路径
		if strings.HasPrefix(trimmed, "//") {
			return "", errs.CodeInvalidParam.New("coverUrl 不允许协议相对地址")
		}
		return trimmed, nil
	}
	return "", errs.CodeInvalidParam.New("coverUrl 必须是 http(s) 地址或站内相对路径")
}
