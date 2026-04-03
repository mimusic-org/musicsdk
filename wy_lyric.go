//go:build wasip1

package musicsdk

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
)

// WyLyricFetcher wy 平台歌词获取器
type WyLyricFetcher struct{}

// NewWyLyricFetcher 创建 wy 平台歌词获取器
func NewWyLyricFetcher() *WyLyricFetcher {
	return &WyLyricFetcher{}
}

// ID 返回平台 ID
func (f *WyLyricFetcher) ID() string {
	return "wy"
}

// wyLyricParams 歌词请求参数
type wyLyricParams struct {
	ID  string `json:"id"`
	Cp  bool   `json:"cp"`
	Tv  int    `json:"tv"`
	Lv  int    `json:"lv"`
	Rv  int    `json:"rv"`
	Kv  int    `json:"kv"`
	Yv  int    `json:"yv"`
	Ytv int    `json:"ytv"`
	Yrv int    `json:"yrv"`
}

// wyLyricResponse 歌词 API 响应
type wyLyricResponse struct {
	Code     int        `json:"code"`
	Lrc      *wyLrcData `json:"lrc"`
	Tlyric   *wyLrcData `json:"tlyric"`
	Romalrc  *wyLrcData `json:"romalrc"`
	Yrc      *wyLrcData `json:"yrc"`      // 逐字歌词
	Ytlrc    *wyLrcData `json:"ytlrc"`    // 逐字翻译歌词
	Yromalrc *wyLrcData `json:"yromalrc"` // 逐字罗马音歌词
}

type wyLrcData struct {
	Lyric string `json:"lyric"`
}

// GetLyric 获取歌词
// songInfo 需要包含 musicId 字段
func (f *WyLyricFetcher) GetLyric(songInfo map[string]interface{}) (*LyricResult, error) {
	// 获取 musicId
	var musicId string
	switch v := songInfo["musicId"].(type) {
	case string:
		musicId = v
	case float64:
		musicId = strconv.FormatInt(int64(v), 10)
	case int64:
		musicId = strconv.FormatInt(v, 10)
	case int:
		musicId = strconv.Itoa(v)
	}
	if musicId == "" {
		return nil, fmt.Errorf("missing musicId in songInfo")
	}

	// 构造请求参数
	params := wyLyricParams{
		ID:  musicId,
		Cp:  false,
		Tv:  0,
		Lv:  0,
		Rv:  0,
		Kv:  0,
		Yv:  0,
		Ytv: 0,
		Yrv: 0,
	}

	// eapi 加密
	encryptedParams, err := eapiEncrypt("/api/song/lyric/v1", params)
	if err != nil {
		return nil, fmt.Errorf("encrypt params: %w", err)
	}

	// 构造表单数据
	formData := "params=" + encryptedParams
	apiURL := "https://interface3.music.163.com/eapi/song/lyric/v1"

	slog.Info("wy lyric request", "musicId", musicId, "url", apiURL)

	// 发送请求
	headers := map[string]string{
		"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.90 Safari/537.36",
		"Origin":     "https://music.163.com",
	}
	respBytes, err := HTTPPostForm(apiURL, []byte(formData), headers)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}

	slog.Debug("wy lyric response", "respLen", len(respBytes))

	// 解析响应
	var resp wyLyricResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	// 检查响应状态
	if resp.Code != 200 {
		return nil, fmt.Errorf("api error: code=%d", resp.Code)
	}

	if resp.Lrc == nil || resp.Lrc.Lyric == "" {
		return nil, fmt.Errorf("no lyric found")
	}

	// 解析歌词
	result := f.parseLyrics(&resp)

	slog.Info("wy lyric success", "musicId", musicId, "lyricLen", len(result.Lyric), "tlyricLen", len(result.TLyric))

	return result, nil
}

// parseLyrics 解析歌词响应
func (f *WyLyricFetcher) parseLyrics(resp *wyLyricResponse) *LyricResult {
	result := &LyricResult{}

	// 修复时间标签格式 [MM:SS:ms] -> [MM:SS.ms]
	fixTimeLabel := func(lrc string) string {
		if lrc == "" {
			return lrc
		}
		re := regexp.MustCompile(`\[(\d{2}:\d{2}):(\d{2,3})\]`)
		return re.ReplaceAllString(lrc, "[$1.$2]")
	}

	// 优先使用逐字歌词 (yrc)
	if resp.Yrc != nil && resp.Yrc.Lyric != "" {
		parsed := f.parseYrcLyric(resp.Yrc.Lyric)
		result.Lyric = parsed.Lyric
		result.LxLyric = parsed.LxLyric

		// 解析逐字翻译歌词
		if resp.Ytlrc != nil && resp.Ytlrc.Lyric != "" {
			tlyric := f.parseHeaderInfo(resp.Ytlrc.Lyric)
			if tlyric != "" {
				result.TLyric = f.fixTimeTag(result.Lyric, tlyric)
			}
		}

		// 解析逐字罗马音歌词
		if resp.Yromalrc != nil && resp.Yromalrc.Lyric != "" {
			rlyric := f.parseHeaderInfo(resp.Yromalrc.Lyric)
			if rlyric != "" {
				result.RLyric = f.fixTimeTag(result.Lyric, rlyric)
			}
		}
	} else {
		// 使用普通歌词
		if resp.Lrc != nil && resp.Lrc.Lyric != "" {
			result.Lyric = fixTimeLabel(f.parseHeaderInfo(resp.Lrc.Lyric))
		}

		if resp.Tlyric != nil && resp.Tlyric.Lyric != "" {
			result.TLyric = fixTimeLabel(f.parseHeaderInfo(resp.Tlyric.Lyric))
		}

		if resp.Romalrc != nil && resp.Romalrc.Lyric != "" {
			result.RLyric = fixTimeLabel(f.parseHeaderInfo(resp.Romalrc.Lyric))
		}
	}

	return result
}

// parseHeaderInfo 解析头部信息（处理 JSON 格式的行）
func (f *WyLyricFetcher) parseHeaderInfo(str string) string {
	str = strings.TrimSpace(str)
	str = strings.ReplaceAll(str, "\r", "")
	if str == "" {
		return ""
	}

	lines := strings.Split(str, "\n")
	var result []string

	infoRe := regexp.MustCompile(`^{\"`)

	for _, line := range lines {
		if !infoRe.MatchString(line) {
			result = append(result, line)
			continue
		}

		// 尝试解析 JSON 格式的行
		parsed := f.parseJsonLine(line)
		if parsed != "" {
			result = append(result, parsed)
		}
	}

	return strings.Join(result, "\n")
}

// parseJsonLine 解析 JSON 格式的歌词行
func (f *WyLyricFetcher) parseJsonLine(line string) string {
	var info struct {
		T int `json:"t"`
		C []struct {
			Tx string `json:"tx"`
		} `json:"c"`
	}

	if err := json.Unmarshal([]byte(line), &info); err != nil {
		return ""
	}

	timeTag := f.msFormat(info.T)
	if timeTag == "" {
		return ""
	}

	var words []string
	for _, c := range info.C {
		words = append(words, c.Tx)
	}

	return timeTag + strings.Join(words, "")
}

// parseYrcLyric 解析逐字歌词 (yrc)
func (f *WyLyricFetcher) parseYrcLyric(str string) struct{ Lyric, LxLyric string } {
	str = strings.TrimSpace(str)
	str = strings.ReplaceAll(str, "\r", "")

	lines := f.parseHeaderInfo(str)
	if lines == "" {
		return struct{ Lyric, LxLyric string }{}
	}

	lineTimeRe := regexp.MustCompile(`^\[(\d+),\d+\]`)
	wordTimeRe := regexp.MustCompile(`\(\d+,\d+,\d+\)`)
	wordTimeAllRe := regexp.MustCompile(`(\(\d+,\d+,\d+\))`)

	var lrcLines, lxlrcLines []string

	for _, line := range strings.Split(lines, "\n") {
		line = strings.TrimSpace(line)
		result := lineTimeRe.FindStringSubmatch(line)
		if result == nil {
			if strings.HasPrefix(line, "[offset") {
				lxlrcLines = append(lxlrcLines, line)
				lrcLines = append(lrcLines, line)
			}
			continue
		}

		startMsTime, _ := strconv.Atoi(result[1])
		startTimeStr := f.msFormat(startMsTime)
		if startTimeStr == "" {
			continue
		}

		words := lineTimeRe.ReplaceAllString(line, "")
		lrcLines = append(lrcLines, startTimeStr+wordTimeAllRe.ReplaceAllString(words, ""))

		times := wordTimeAllRe.FindAllString(words, -1)
		if times == nil {
			continue
		}

		// 转换逐字时间标签
		var newTimes []string
		for _, time := range times {
			re := regexp.MustCompile(`\((\d+),(\d+),\d+\)`)
			m := re.FindStringSubmatch(time)
			if m != nil {
				t1, _ := strconv.Atoi(m[1])
				t2, _ := strconv.Atoi(m[2])
				newTimes = append(newTimes, fmt.Sprintf("<%d,%d>", max(t1-startMsTime, 0), t2))
			}
		}

		wordArr := wordTimeRe.Split(words, -1)
		if len(wordArr) > 0 {
			wordArr = wordArr[1:] // 移除第一个空元素
		}

		var newWords strings.Builder
		for i, t := range newTimes {
			newWords.WriteString(t)
			if i < len(wordArr) {
				newWords.WriteString(wordArr[i])
			}
		}
		lxlrcLines = append(lxlrcLines, startTimeStr+newWords.String())
	}

	return struct{ Lyric, LxLyric string }{
		Lyric:   strings.Join(lrcLines, "\n"),
		LxLyric: strings.Join(lxlrcLines, "\n"),
	}
}

// msFormat 毫秒格式化为时间标签 [MM:SS.ms]
func (f *WyLyricFetcher) msFormat(timeMs int) string {
	if timeMs < 0 {
		return ""
	}
	ms := timeMs % 1000
	timeMs /= 1000
	m := timeMs / 60
	s := timeMs % 60
	return fmt.Sprintf("[%02d:%02d.%d]", m, s, ms)
}

// getIntv 解析时间标签为毫秒
func (f *WyLyricFetcher) getIntv(interval string) int {
	if interval == "" {
		return 0
	}
	if !strings.Contains(interval, ".") {
		interval += ".0"
	}
	parts := strings.FieldsFunc(interval, func(r rune) bool {
		return r == ':' || r == '.'
	})
	for len(parts) < 3 {
		parts = append([]string{"0"}, parts...)
	}
	m, _ := strconv.Atoi(parts[0])
	s, _ := strconv.Atoi(parts[1])
	ms, _ := strconv.Atoi(parts[2])
	return m*60000 + s*1000 + ms
}

// fixTimeTag 修复翻译歌词时间标签
func (f *WyLyricFetcher) fixTimeTag(lrc, targetLrc string) string {
	timeRe := regexp.MustCompile(`^\[([\d:.]+)\]`)

	lrcLines := strings.Split(lrc, "\n")
	targetLines := strings.Split(targetLrc, "\n")

	var temp []string
	var newLrc []string

	for _, line := range targetLines {
		result := timeRe.FindStringSubmatch(line)
		if result == nil {
			continue
		}
		words := timeRe.ReplaceAllString(line, "")
		if strings.TrimSpace(words) == "" {
			continue
		}
		t1 := f.getIntv(result[1])

		for len(lrcLines) > 0 {
			lrcLine := lrcLines[0]
			lrcLines = lrcLines[1:]

			lrcResult := timeRe.FindStringSubmatch(lrcLine)
			if lrcResult == nil {
				continue
			}
			t2 := f.getIntv(lrcResult[1])

			if abs(t1-t2) < 100 {
				newLine := timeRe.ReplaceAllString(line, lrcResult[0])
				newLine = strings.TrimSpace(newLine)
				if newLine != "" {
					newLrc = append(newLrc, newLine)
				}
				break
			}
			temp = append(temp, lrcLine)
		}
		lrcLines = append(temp, lrcLines...)
		temp = nil
	}

	return strings.Join(newLrc, "\n")
}

// abs 返回绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
