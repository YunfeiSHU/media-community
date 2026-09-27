// Package hub 是弹幕服务的 WebSocket 核心：连接管理 + 房间广播。
//
// 为什么单独成包：这是本服务最核心、最容易被写坏的并发代码
// （map + channel + mutex 三件套）。独立成包后可以单独写并发测试（`-race`），
// 也避免业务代码里混入连接管理细节。
//
// 阶段一是**单实例广播**：连接在哪个实例，就只能收到该实例 Hub 的广播。
// 多实例部署时同房间观众会被负载均衡分到不同实例而互相收不到弹幕 ——
// 这是已知边界，`Backend` 接口就是为此预留的扩展点（设计文档 §4.5）。
package hub

import (
	"encoding/json"
	"fmt"
	"time"
)

// 帧类型（服务端 ↔ 客户端）。
const (
	// TypeWelcome 连接建立后服务端首帧。
	TypeWelcome = "welcome"
	// TypeDanmaku 弹幕广播。
	TypeDanmaku = "danmaku"
	// TypeSystem 系统消息（直播开始/结束、服务端关闭）。
	TypeSystem = "system"
	// TypePing 客户端心跳请求。
	TypePing = "ping"
	// TypePong 服务端心跳响应。
	TypePong = "pong"
	// TypeError 发送失败等错误（**不关闭连接**）。
	TypeError = "error"
)

// 系统消息事件名。
const (
	EventLiveStarted    = "LIVE_STARTED"
	EventLiveEnded      = "LIVE_ENDED"
	EventServerShutdown = "SERVER_SHUTDOWN"
)

// 关闭码。
const (
	// CloseNormalClosure 正常关闭。
	CloseNormalClosure = 1000
	// CloseGoingAway 服务端退出 / 读超时。
	CloseGoingAway = 1001
	// CloseUnsupportedData 连续协议解析失败。
	CloseUnsupportedData = 1003
	// ClosePolicyViolation 帧过大等策略问题。
	ClosePolicyViolation = 1008
	// CloseKickedByNewConn 同一 uid 建立新连接，踢掉最旧连接（应用自定义码）。
	CloseKickedByNewConn = 4001
)

// Frame 是 WebSocket 的统一帧格式：{"type":"...","data":{...}}。
type Frame struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// NewFrame 把 data 序列化后装进帧。
func NewFrame(typ string, data any) ([]byte, error) {
	if data == nil {
		return json.Marshal(Frame{Type: typ})
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("序列化帧数据失败: %w", err)
	}
	return json.Marshal(Frame{Type: typ, Data: raw})
}

// MustFrame 与 NewFrame 相同，但序列化失败时返回 nil（调用方丢弃该帧）。
//
// 广播路径上序列化失败只应影响这一帧，不应该放大成请求级错误。
func MustFrame(typ string, data any) []byte {
	raw, err := NewFrame(typ, data)
	if err != nil {
		return nil
	}
	return raw
}

// DecodeFrame 解析客户端帧。
func DecodeFrame(raw []byte) (Frame, error) {
	var f Frame
	if err := json.Unmarshal(raw, &f); err != nil {
		return f, fmt.Errorf("帧不是合法 JSON: %w", err)
	}
	if f.Type == "" {
		return f, fmt.Errorf("帧缺少 type 字段")
	}
	return f, nil
}

// Bind 把帧的 data 反序列化到 v（data 为空时返回 nil，由调用方决定是否必填）。
func (f Frame) Bind(v any) error {
	if len(f.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(f.Data, v); err != nil {
		return fmt.Errorf("解析 %s 帧数据失败: %w", f.Type, err)
	}
	return nil
}

// WelcomeData 是 welcome 帧的载荷。
type WelcomeData struct {
	ClientID    string    `json:"clientId"`
	RoomID      string    `json:"roomId"`
	VideoID     string    `json:"videoId"`
	UID         string    `json:"uid"`
	ReadOnly    bool      `json:"readOnly"`
	OnlineCount int       `json:"onlineCount"`
	HotCount    int64     `json:"hotCount"`
	ServerTime  time.Time `json:"serverTime"`
}

// SystemData 是 system 帧的载荷。
type SystemData struct {
	Event   string    `json:"event"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// ErrorData 是 error 帧的载荷。RetryAfterMs 仅限流错误携带。
type ErrorData struct {
	Code         int    `json:"code"`
	Message      string `json:"message"`
	RetryAfterMs int64  `json:"retryAfterMs,omitempty"`
}

// PongData 是 pong 帧的载荷。
type PongData struct {
	ServerTime time.Time `json:"serverTime"`
}

// InboundDanmaku 是客户端 danmaku 帧的载荷。
//
// roomId / videoId **不在帧内**：以握手参数为准，防止跨房间注入。
type InboundDanmaku struct {
	Content     string `json:"content"`
	Color       string `json:"color"`
	Position    string `json:"position"`
	TimeMs      int    `json:"timeMs"`
	ClientMsgID string `json:"clientMsgId"`
}
