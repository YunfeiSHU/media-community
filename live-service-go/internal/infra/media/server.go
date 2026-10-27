// Package media 抽象「媒体服务」这一外部依赖。
//
// 为什么需要一层抽象：本服务是**控制面**，不处理音视频。
// 真正的收流/转协议由 SRS 承担，而 SRS 在本地开发时往往没有，
// 因此把「拼推拉流地址」与「主动断流」收敛到 Server 接口，
// 用 mock / srs 两种实现切换（对齐 video-service 的 Transcoder 设计）。
package media

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/example/live-service-go/internal/config"
)

// PlayURLs 是各协议播放地址。
type PlayURLs struct {
	HLS     string `json:"hls"`
	HTTPFLV string `json:"httpFlv"`
	WebRTC  string `json:"webrtc"`
	RTMP    string `json:"rtmp"`
}

// Server 是媒体服务适配器。
type Server interface {
	// Name 返回实现名（mock | srs）。
	Name() string
	// PushURL 返回主播推流地址。
	//
	// sessionID 会作为 query 参数带上：SRS 会在 on_publish 回调里原样回传，
	// 从而让服务端能识别「这次回调属于哪一次开播」，杜绝旧会话的延迟回调。
	PushURL(streamKey, sessionID string) string
	// PlayURLs 返回各协议播放地址。
	PlayURLs(streamKey string) PlayURLs
	// KickOff 主动断开某路推流（只对真实 SRS 有效）。
	KickOff(ctx context.Context, streamKey string) error
	// Health 探活；mock 实现恒返回 nil。
	Health(ctx context.Context) error
}

// New 按配置构造媒体服务适配器。
func New(cfg config.Media) Server {
	if cfg.Driver == "srs" {
		return NewSRS(cfg)
	}
	return NewMock(cfg)
}

// urls 根据配置拼装地址；mock 与 srs 实现共用（差异只在「是否真的连 SRS」）。
type urls struct {
	cfg config.Media
}

// PushURL 拼装推流地址。
func (u urls) PushURL(streamKey, sessionID string) string {
	base := fmt.Sprintf("rtmp://%s:%d/%s/%s", u.cfg.RTMPHost, u.cfg.RTMPPort, u.cfg.App, streamKey)
	if sessionID == "" {
		return base
	}
	return base + "?session=" + url.QueryEscape(sessionID)
}

// PlayURLs 拼装四个协议的播放地址。
//
// 设置了 media.public_base 时，HLS / FLV 一律用它拼（同源反代场景），
// 否则退回 http://rtmp_host:http_port（本地直连调试）。
// 注意反代侧要剥掉 /live 前缀：/live/live/<key>.m3u8 -> SRS 的 /live/<key>.m3u8。
func (u urls) PlayURLs(streamKey string) PlayURLs {
	httpBase := fmt.Sprintf("http://%s:%d/%s", u.cfg.RTMPHost, u.cfg.HTTPPort, u.cfg.App)
	if base := strings.TrimRight(u.cfg.PublicBase, "/"); base != "" {
		httpBase = base + "/" + u.cfg.App
	}
	rtmpBase := fmt.Sprintf("rtmp://%s:%d/%s", u.cfg.RTMPHost, u.cfg.RTMPPort, u.cfg.App)
	return PlayURLs{
		HLS:     fmt.Sprintf("%s/%s.m3u8", httpBase, streamKey),
		HTTPFLV: fmt.Sprintf("%s/%s.flv", httpBase, streamKey),
		WebRTC:  fmt.Sprintf("webrtc://%s:%d/%s/%s", u.cfg.WebRTCHost, u.cfg.WebRTCPort, u.cfg.App, streamKey),
		RTMP:    fmt.Sprintf("%s/%s", rtmpBase, streamKey),
	}
}

// SessionFromParam 从 SRS 回调的 param 字段（形如 "?session=xxx&secret=yyy"）里取值。
//
// SRS 把推流地址的 query 原样放进 param，因此这里只做最小的解析：
// 加 "?" 前缀后再用 net/url 解析，避免业务代码到处手写字符串切割。
func SessionFromParam(param string) string {
	values, err := url.ParseQuery(trimQuestion(param))
	if err != nil {
		return ""
	}
	return values.Get("session")
}

// ParamValue 从 SRS 回调的 param 里取任意 query 参数。
func ParamValue(param, key string) string {
	values, err := url.ParseQuery(trimQuestion(param))
	if err != nil {
		return ""
	}
	return values.Get(key)
}

func trimQuestion(in string) string {
	if len(in) > 0 && in[0] == '?' {
		return in[1:]
	}
	return in
}
