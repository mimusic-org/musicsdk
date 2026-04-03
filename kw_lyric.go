//go:build wasip1

package musicsdk

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// KwLyricFetcher kw 平台歌词获取器
type KwLyricFetcher struct{}

// NewKwLyricFetcher 创建 kw 平台歌词获取器
func NewKwLyricFetcher() *KwLyricFetcher {
	return &KwLyricFetcher{}
}

// ID 返回平台 ID
func (f *KwLyricFetcher) ID() string {
	return "kw"
}

// kwLyricResponse kw 歌词 API 响应
type kwLyricResponse struct {
	Data struct {
		Songinfo struct {
			SongName string `json:"songName"`
			Artist   string `json:"artist"`
			Album    string `json:"album"`
		} `json:"songinfo"`
		Lrclist []struct {
			Time      float64 `json:"time"`
			LineLyric string  `json:"lineLyric"`
		} `json:"lrclist"`
	} `json:"data"`
}

// GetLyric 获取歌词
// songInfo 需要包含 musicId 或 songmid 字段
func (f *KwLyricFetcher) GetLyric(songInfo map[string]interface{}) (*LyricResult, error) {
	// 获取 musicId
	var musicId string
	if mid, ok := songInfo["musicId"].(string); ok && mid != "" {
		musicId = mid
	} else if mid, ok := songInfo["songmid"].(string); ok && mid != "" {
		musicId = mid
	} else {
		// 尝试数字类型
		switch v := songInfo["musicId"].(type) {
		case float64:
			musicId = strconv.FormatInt(int64(v), 10)
		case int64:
			musicId = strconv.FormatInt(v, 10)
		case int:
			musicId = strconv.Itoa(v)
		}
	}

	if musicId == "" {
		return nil, fmt.Errorf("missing musicId in songInfo")
	}

	// 使用移动端 API 获取歌词（无需加密）
	apiURL := fmt.Sprintf("http://m.kuwo.cn/newh5/singles/songinfoandlrc?musicId=%s", musicId)

	slog.Info("kw lyric request", "musicId", musicId, "url", apiURL)

	respBytes, err := HTTPGet(apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}

	slog.Debug("kw lyric response", "respLen", len(respBytes))

	// 解析响应
	var resp kwLyricResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	if len(resp.Data.Lrclist) == 0 {
		return nil, fmt.Errorf("no lyric found")
	}

	// 转换歌词格式
	lyricInfo := f.parseLrcList(resp.Data.Lrclist, resp.Data.Songinfo)

	slog.Info("kw lyric success", "musicId", musicId, "lyricLen", len(lyricInfo.Lyric), "tlyricLen", len(lyricInfo.TLyric))

	return lyricInfo, nil
}

// parseLrcList 解析歌词列表
func (f *KwLyricFetcher) parseLrcList(lrclist []struct {
	Time      float64 `json:"time"`
	LineLyric string  `json:"lineLyric"`
}, songinfo struct {
	SongName string `json:"songName"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
}) *LyricResult {
	// 排序并分离主歌词和翻译歌词
	lrc, tlyric := f.sortLrcArr(lrclist)

	// 生成头部标签
	tags := []string{
		fmt.Sprintf("[ti:%s]", songinfo.SongName),
		fmt.Sprintf("[ar:%s]", songinfo.Artist),
		fmt.Sprintf("[al:%s]", songinfo.Album),
		"[by:]",
		"[offset:0]",
	}

	// 转换为 LRC 格式
	lyric := f.transformLrc(tags, lrc)
	var tlyricStr string
	if len(tlyric) > 0 {
		tlyricStr = f.transformLrc(tags, tlyric)
	}

	return &LyricResult{
		Lyric:  DecodeName(lyric),
		TLyric: DecodeName(tlyricStr),
	}
}

// sortLrcArr 分离主歌词和翻译歌词
func (f *KwLyricFetcher) sortLrcArr(arr []struct {
	Time      float64 `json:"time"`
	LineLyric string  `json:"lineLyric"`
}) (lrc, tlyric []struct {
	Time string
	Text string
}) {
	seen := make(map[string]bool)

	for _, item := range arr {
		timeStr := f.formatTime(item.Time)

		if seen[timeStr] {
			// 时间重复，可能是翻译歌词
			if len(lrc) < 2 {
				continue
			}
			// 把上一行移到翻译歌词
			lastItem := lrc[len(lrc)-1]
			lrc = lrc[:len(lrc)-1]
			if len(lrc) > 0 {
				lastItem.Time = lrc[len(lrc)-1].Time
			}
			tlyric = append(tlyric, lastItem)
			lrc = append(lrc, struct {
				Time string
				Text string
			}{timeStr, item.LineLyric})
		} else {
			lrc = append(lrc, struct {
				Time string
				Text string
			}{timeStr, item.LineLyric})
			seen[timeStr] = true
		}
	}

	return lrc, tlyric
}

// formatTime 格式化时间
func (f *KwLyricFetcher) formatTime(time float64) string {
	m := int(time) / 60
	s := time - float64(m*60)
	return fmt.Sprintf("%02d:%05.2f", m, s)
}

// transformLrc 转换为 LRC 格式
func (f *KwLyricFetcher) transformLrc(tags []string, lrclist []struct {
	Time string
	Text string
}) string {
	var lines []string
	lines = append(lines, tags...)

	if len(lrclist) == 0 {
		lines = append(lines, "暂无歌词")
	} else {
		for _, l := range lrclist {
			lines = append(lines, fmt.Sprintf("[%s]%s", l.Time, l.Text))
		}
	}

	return strings.Join(lines, "\n")
}
