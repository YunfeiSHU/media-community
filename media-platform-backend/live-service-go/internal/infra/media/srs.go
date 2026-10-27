package media

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/example/live-service-go/internal/config"
)

// SRSServer 是真实 SRS 实现：地址按配置拼装，断流通过 SRS HTTP API。
type SRSServer struct {
	urls
	client *http.Client
}

// NewSRS 构造 SRS 适配器。
func NewSRS(cfg config.Media) *SRSServer {
	return &SRSServer{
		urls:   urls{cfg: cfg},
		client: &http.Client{Timeout: 3 * time.Second},
	}
}

// Name 返回实现名。
func (s *SRSServer) Name() string { return "srs" }

// KickOff 主动断开某路推流。
//
// SRS 5 的 HTTP API 没有「按 stream 断流」的接口，需要两步：
//  1. GET /api/v1/streams 找到 name == streamKey 的流；
//  2. DELETE /api/v1/clients/{clientId} 逐个断开。
//
// 流不存在时返回 nil（说明主播已经断了，属于正常情况而非错误）。
func (s *SRSServer) KickOff(ctx context.Context, streamKey string) error {
	clientIDs, err := s.findClients(ctx, streamKey)
	if err != nil {
		return err
	}
	for _, id := range clientIDs {
		endpoint := fmt.Sprintf("%s/api/v1/clients/%s", s.base(), id)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
		if err != nil {
			return fmt.Errorf("构造断流请求失败: %w", err)
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("调用 SRS 断流接口失败: %w", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= http.StatusBadRequest {
			return fmt.Errorf("SRS 断流接口返回 HTTP %d", resp.StatusCode)
		}
	}
	return nil
}

// Health 通过 SRS 版本接口探活。
func (s *SRSServer) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base()+"/api/v1/versions", nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("SRS 探活失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("SRS 探活返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

// findClients 找出推该路流的 client id 列表。
func (s *SRSServer) findClients(ctx context.Context, streamKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base()+"/api/v1/streams", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("查询 SRS 流列表失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var payload struct {
		Streams []struct {
			Name    string `json:"name"`
			App     string `json:"app"`
			Clients []struct {
				ID string `json:"id"`
			} `json:"clients"`
		} `json:"streams"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析 SRS 流列表失败: %w", err)
	}

	var ids []string
	for _, stream := range payload.Streams {
		if stream.Name != streamKey || stream.App != s.cfg.App {
			continue
		}
		for _, client := range stream.Clients {
			if client.ID != "" {
				ids = append(ids, client.ID)
			}
		}
	}
	return ids, nil
}

func (s *SRSServer) base() string {
	base := strings.TrimSpace(s.cfg.APIBase)
	if base == "" {
		base = "http://127.0.0.1:1986"
	}
	return strings.TrimRight(base, "/")
}
