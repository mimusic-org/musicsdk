//go:build wasip1

package musicsdk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

// KgLyricFetcher 酷狗音乐歌词获取器
type KgLyricFetcher struct{}

// NewKgLyricFetcher 创建酷狗音乐歌词获取器
func NewKgLyricFetcher() *KgLyricFetcher {
	return &KgLyricFetcher{}
}

// ID 返回平台 ID
func (f *KgLyricFetcher) ID() string {
	return "kg"
}

// kgLyricSearchResponse 歌词搜索响应
type kgLyricSearchResponse struct {
	Candidates []struct {
		ID          string `json:"id"`
		AccessKey   string `json:"accesskey"`
		KrcType     int    `json:"krctype"`
		ContentType int    `json:"contenttype"`
	} `json:"candidates"`
}

// kgLyricDownloadResponse 歌词下载响应
type kgLyricDownloadResponse struct {
	Fmt     string `json:"fmt"`
	Content string `json:"content"` // base64 编码的歌词
}

// 酷狗歌词请求头
var kgLyricHeaders = map[string]string{
	"KG-RC":      "1",
	"KG-THash":   "expand_search_manager.cpp:852736169:451",
	"User-Agent": "KuGou2012-9020-ExpandSearchManager",
}

// GetLyric 获取歌词
// songInfo 需要包含 name, singer, hash, duration 字段
func (f *KgLyricFetcher) GetLyric(songInfo map[string]interface{}) (*LyricResult, error) {
	// 获取必要字段
	name, _ := songInfo["name"].(string)
	singer, _ := songInfo["singer"].(string)
	hash, _ := songInfo["hash"].(string)

	if hash == "" {
		return nil, fmt.Errorf("missing hash in songInfo")
	}

	// 获取时长（毫秒）
	duration := f.getDuration(songInfo)

	// 构建搜索关键词
	keyword := name
	if singer != "" {
		keyword = name + "-" + singer
	}

	// 第一步：搜索歌词
	searchResult, err := f.searchLyric(keyword, hash, duration)
	if err != nil {
		return nil, fmt.Errorf("search lyric failed: %w", err)
	}

	if searchResult == nil {
		return nil, fmt.Errorf("no lyric found")
	}

	// 第二步：下载歌词
	return f.downloadLyric(searchResult.ID, searchResult.AccessKey, searchResult.Fmt)
}

// getDuration 获取时长（毫秒）
func (f *KgLyricFetcher) getDuration(songInfo map[string]interface{}) int {
	// 尝试从 _interval 获取（已是毫秒）
	if interval, ok := songInfo["_interval"]; ok {
		switch v := interval.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		}
	}

	// 尝试从 duration 获取（秒）
	if duration, ok := songInfo["duration"]; ok {
		switch v := duration.(type) {
		case float64:
			return int(v) * 1000
		case int:
			return v * 1000
		case int64:
			return int(v) * 1000
		}
	}

	// 尝试从 interval 获取（MM:SS 格式）
	if interval, ok := songInfo["interval"].(string); ok {
		return f.parseInterval(interval) * 1000
	}

	return 0
}

// parseInterval 解析 MM:SS 格式的时长
func (f *KgLyricFetcher) parseInterval(interval string) int {
	if interval == "" {
		return 0
	}
	parts := strings.Split(interval, ":")
	total := 0
	unit := 1
	for i := len(parts) - 1; i >= 0; i-- {
		v, _ := strconv.Atoi(parts[i])
		total += v * unit
		unit *= 60
	}
	return total
}

type lyricSearchResult struct {
	ID        string
	AccessKey string
	Fmt       string
}

// searchLyric 搜索歌词
func (f *KgLyricFetcher) searchLyric(keyword, hash string, duration int) (*lyricSearchResult, error) {
	params := url.Values{}
	params.Set("ver", "1")
	params.Set("man", "yes")
	params.Set("client", "pc")
	params.Set("keyword", keyword)
	params.Set("hash", hash)
	params.Set("timelength", strconv.Itoa(duration))
	params.Set("lrctxt", "1")

	apiURL := "http://lyrics.kugou.com/search?" + params.Encode()

	slog.Info("kg lyric search", "keyword", keyword, "hash", hash, "url", apiURL)

	respBytes, err := HTTPGet(apiURL, kgLyricHeaders)
	if err != nil {
		return nil, err
	}

	var resp kgLyricSearchResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 {
		return nil, nil
	}

	candidate := resp.Candidates[0]

	// 确定歌词格式：krctype == 1 且 contenttype != 1 时为 krc，否则为 lrc
	fmt := "lrc"
	if candidate.KrcType == 1 && candidate.ContentType != 1 {
		fmt = "krc"
	}

	return &lyricSearchResult{
		ID:        candidate.ID,
		AccessKey: candidate.AccessKey,
		Fmt:       fmt,
	}, nil
}

// downloadLyric 下载歌词
func (f *KgLyricFetcher) downloadLyric(id, accessKey, lyricFmt string) (*LyricResult, error) {
	params := url.Values{}
	params.Set("ver", "1")
	params.Set("client", "pc")
	params.Set("id", id)
	params.Set("accesskey", accessKey)
	params.Set("fmt", lyricFmt)
	params.Set("charset", "utf8")

	apiURL := "http://lyrics.kugou.com/download?" + params.Encode()

	slog.Info("kg lyric download", "id", id, "fmt", lyricFmt, "url", apiURL)

	respBytes, err := HTTPGet(apiURL, kgLyricHeaders)
	if err != nil {
		return nil, err
	}

	var resp kgLyricDownloadResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}

	switch resp.Fmt {
	case "krc":
		// KRC 格式需要解密，当前简化版直接返回空
		// TODO: 实现 KRC 解密
		slog.Warn("kg lyric krc format not supported yet, returning empty")
		return &LyricResult{}, nil
	case "lrc":
		// LRC 格式直接 base64 解码
		decoded, err := base64.StdEncoding.DecodeString(resp.Content)
		if err != nil {
			return nil, fmt.Errorf("decode lyric failed: %w", err)
		}
		lyric := string(decoded)
		slog.Info("kg lyric success", "id", id, "lyricLen", len(lyric))
		return &LyricResult{
			Lyric: lyric,
		}, nil
	default:
		return nil, fmt.Errorf("unknown lyric format: %s", resp.Fmt)
	}
}
