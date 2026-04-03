//go:build wasip1

package musicsdk

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
)

// TxLyricFetcher tx 平台歌词获取器
type TxLyricFetcher struct{}

// NewTxLyricFetcher 创建 tx 平台歌词获取器
func NewTxLyricFetcher() *TxLyricFetcher {
	return &TxLyricFetcher{}
}

// ID 返回平台 ID
func (f *TxLyricFetcher) ID() string {
	return "tx"
}

// txLyricResponse tx 歌词 API 响应
type txLyricResponse struct {
	Code  int    `json:"code"`
	Lyric string `json:"lyric"` // base64 编码的歌词
	Trans string `json:"trans"` // base64 编码的翻译歌词
}

// GetLyric 获取歌词
// songInfo 需要包含 songmid 字段
func (f *TxLyricFetcher) GetLyric(songInfo map[string]interface{}) (*LyricResult, error) {
	// 获取 songmid
	songmid, ok := songInfo["songmid"].(string)
	if !ok || songmid == "" {
		return nil, fmt.Errorf("missing songmid in songInfo")
	}

	// 构建请求 URL
	apiURL := fmt.Sprintf(
		"https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=%s&g_tk=5381&loginUin=0&hostUin=0&format=json&inCharset=utf8&outCharset=utf-8&platform=yqq",
		songmid,
	)

	slog.Info("tx lyric request", "songmid", songmid, "url", apiURL)

	// 设置请求头
	headers := map[string]string{
		"Referer": "https://y.qq.com/portal/player.html",
	}

	// 发送请求
	respBytes, err := HTTPGet(apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}

	slog.Debug("tx lyric response", "respLen", len(respBytes))

	// 解析响应
	var resp txLyricResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	// 检查响应状态
	if resp.Code != 0 {
		return nil, fmt.Errorf("api error: code=%d", resp.Code)
	}

	if resp.Lyric == "" {
		return nil, fmt.Errorf("no lyric found")
	}

	// Base64 解码歌词
	lyric, err := f.decodeBase64Lyric(resp.Lyric)
	if err != nil {
		return nil, fmt.Errorf("decode lyric failed: %w", err)
	}

	// Base64 解码翻译歌词
	var tlyric string
	if resp.Trans != "" {
		tlyric, _ = f.decodeBase64Lyric(resp.Trans)
	}

	slog.Info("tx lyric success", "songmid", songmid, "lyricLen", len(lyric), "tlyricLen", len(tlyric))

	return &LyricResult{
		Lyric:  DecodeName(lyric),
		TLyric: DecodeName(tlyric),
	}, nil
}

// decodeBase64Lyric 解码 Base64 编码的歌词
func (f *TxLyricFetcher) decodeBase64Lyric(encoded string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
