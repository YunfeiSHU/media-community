package media

import (
	"testing"

	"github.com/example/live-service-go/internal/config"
)

func testURLs(publicBase string) urls {
	return urls{cfg: config.Media{
		App:        "live",
		RTMPHost:   "srs.example.com",
		RTMPPort:   1935,
		HTTPPort:   8088,
		WebRTCHost: "srs.example.com",
		WebRTCPort: 8000,
		PublicBase: publicBase,
	}}
}

func TestPlayURLsUsesPublicBase(t *testing.T) {
	got := testURLs("https://site/live/").PlayURLs("key1")

	// 末尾斜杠要归一，否则会出现 https://site/live//live/key1.m3u8
	if want := "https://site/live/live/key1.m3u8"; got.HLS != want {
		t.Fatalf("HLS = %q, 期望 %q", got.HLS, want)
	}
	if want := "https://site/live/live/key1.flv"; got.HTTPFLV != want {
		t.Fatalf("HTTPFLV = %q, 期望 %q", got.HTTPFLV, want)
	}
}

func TestPlayURLsFallsBackToDirectSRS(t *testing.T) {
	got := testURLs("").PlayURLs("key1")

	// 没配 public_base 时保持旧行为：直连 SRS（仅本地调试可用）
	if want := "http://srs.example.com:8088/live/key1.m3u8"; got.HLS != want {
		t.Fatalf("HLS = %q, 期望 %q", got.HLS, want)
	}
}
