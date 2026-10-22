// Package media 封装媒体处理能力（转码）。
//
// 之所以抽象成接口：转码是本项目最脆弱的一环（依赖外部二进制、耗时长、可能失败），
// 抽象后可以：
//   - 在没有 FFmpeg 的机器上用 mock 驱动跑通完整业务链路
//   - 在单元测试里注入假实现
//   - 未来替换成云转码服务（如阿里云 MPS）而不动业务代码
package media

import "strings"

// Profile 描述一个转码档位。
type Profile struct {
	Name      string // 360p / 720p / 1080p
	Width     int
	Height    int
	VideoKbps int
	AudioKbps int
}

// Bandwidth 是 HLS 主索引里声明的带宽（bit/s），用于播放器自适应选择。
func (p Profile) Bandwidth() int { return (p.VideoKbps + p.AudioKbps) * 1000 }

// DefaultProfiles 是三档默认转码配置（与《接口文档》里 sources 的三档一致）。
var DefaultProfiles = []Profile{
	{Name: "360p", Width: 640, Height: 360, VideoKbps: 700, AudioKbps: 96},
	{Name: "720p", Width: 1280, Height: 720, VideoKbps: 2400, AudioKbps: 128},
	{Name: "1080p", Width: 1920, Height: 1080, VideoKbps: 4800, AudioKbps: 128},
}

// ProfileNames 返回全部档位名。
func ProfileNames() []string {
	names := make([]string, 0, len(DefaultProfiles))
	for _, p := range DefaultProfiles {
		names = append(names, p.Name)
	}
	return names
}

// JoinProfileNames 把档位名拼成入库用的字符串（如 "360p,720p,1080p"）。
func JoinProfileNames() string { return strings.Join(ProfileNames(), ",") }

// SplitProfileNames 解析入库的档位字符串。
func SplitProfileNames(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// buildMasterPlaylist 生成多码率主索引（master.m3u8）。
func buildMasterPlaylist(uriOf func(Profile) string) string {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n\n")
	for _, p := range DefaultProfiles {
		sb.WriteString("#EXT-X-STREAM-INF:BANDWIDTH=")
		sb.WriteString(itoa(p.Bandwidth()))
		sb.WriteString(",RESOLUTION=")
		sb.WriteString(itoa(p.Width))
		sb.WriteString("x")
		sb.WriteString(itoa(p.Height))
		sb.WriteString("\n")
		sb.WriteString(uriOf(p))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
