//go:build wasip1

package musicsdk

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// MgLyricFetcher mg 平台歌词获取器
type MgLyricFetcher struct{}

// NewMgLyricFetcher 创建 mg 平台歌词获取器
func NewMgLyricFetcher() *MgLyricFetcher {
	return &MgLyricFetcher{}
}

// ID 返回平台 ID
func (f *MgLyricFetcher) ID() string {
	return "mg"
}

// mgMusicInfoResponse 歌曲详情响应
type mgMusicInfoResponse struct {
	Resource []struct {
		LrcURL string `json:"lrcUrl"`
		MrcURL string `json:"mrcUrl"`
		TrcURL string `json:"trcUrl"`
	} `json:"resource"`
}

// mg 歌词请求头
var mgLyricHeaders = map[string]string{
	"Referer":    "https://app.c.nf.migu.cn/",
	"User-Agent": "Mozilla/5.0 (Linux; Android 5.1.1; Nexus 6 Build/LYZ28E) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/59.0.3071.115 Mobile Safari/537.36",
	"channel":    "0146921",
}

// GetLyric 获取歌词
// songInfo 可以包含 lrcUrl/mrcUrl/trcUrl（直接使用），或 copyrightId（需要先获取详情）
func (f *MgLyricFetcher) GetLyric(songInfo map[string]interface{}) (*LyricResult, error) {
	// 检查是否已有歌词 URL
	lrcUrl, _ := songInfo["lrcUrl"].(string)
	mrcUrl, _ := songInfo["mrcUrl"].(string)
	trcUrl, _ := songInfo["trcUrl"].(string)

	// 如果没有歌词 URL，需要先获取歌曲详情
	if lrcUrl == "" && mrcUrl == "" {
		copyrightId, _ := songInfo["copyrightId"].(string)
		if copyrightId == "" {
			return nil, fmt.Errorf("missing copyrightId or lrcUrl/mrcUrl in songInfo")
		}

		// 获取歌曲详情
		info, err := f.getMusicInfo(copyrightId)
		if err != nil {
			return nil, fmt.Errorf("get music info failed: %w", err)
		}
		lrcUrl = info.LrcURL
		mrcUrl = info.MrcURL
		trcUrl = info.TrcURL
	}

	slog.Info("mg lyric urls", "lrcUrl", lrcUrl, "mrcUrl", mrcUrl, "trcUrl", trcUrl)

	// 获取歌词
	var result LyricResult

	// 优先使用 mrcUrl（逐字歌词）
	if mrcUrl != "" {
		mrcInfo, err := f.getMrc(mrcUrl)
		if err == nil {
			result.Lyric = mrcInfo.Lyric
			result.LxLyric = mrcInfo.LxLyric
		} else {
			slog.Warn("get mrc failed, fallback to lrc", "error", err)
		}
	}

	// 如果没有 mrc，使用 lrcUrl
	if result.Lyric == "" && lrcUrl != "" {
		lrcInfo, err := f.getLrc(lrcUrl)
		if err == nil {
			result.Lyric = lrcInfo.Lyric
		} else {
			slog.Warn("get lrc failed", "error", err)
		}
	}

	// 获取翻译歌词
	if trcUrl != "" {
		tlyric, err := f.getText(trcUrl)
		if err == nil {
			result.TLyric = tlyric
		}
	}

	if result.Lyric == "" {
		return nil, fmt.Errorf("no lyric found")
	}

	slog.Info("mg lyric success", "lyricLen", len(result.Lyric), "tlyricLen", len(result.TLyric))

	return &result, nil
}

// getMusicInfo 获取歌曲详情
func (f *MgLyricFetcher) getMusicInfo(copyrightId string) (*struct {
	LrcURL string
	MrcURL string
	TrcURL string
}, error) {
	apiURL := "https://c.musicapp.migu.cn/MIGUM2.0/v1.0/content/resourceinfo.do?resourceType=2"

	// POST 表单数据
	formData := url.Values{}
	formData.Set("resourceId", copyrightId)

	slog.Info("mg get music info", "copyrightId", copyrightId)

	respBytes, err := HTTPPostForm(apiURL, []byte(formData.Encode()), mgLyricHeaders)
	if err != nil {
		return nil, err
	}

	var resp mgMusicInfoResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, err
	}

	if len(resp.Resource) == 0 {
		return nil, fmt.Errorf("no resource found")
	}

	resource := resp.Resource[0]
	return &struct {
		LrcURL string
		MrcURL string
		TrcURL string
	}{
		LrcURL: resource.LrcURL,
		MrcURL: resource.MrcURL,
		TrcURL: resource.TrcURL,
	}, nil
}

// getText 获取文本内容
func (f *MgLyricFetcher) getText(urlStr string) (string, error) {
	if urlStr == "" {
		return "", fmt.Errorf("empty url")
	}

	respBytes, err := HTTPGet(urlStr, mgLyricHeaders)
	if err != nil {
		return "", err
	}

	return string(respBytes), nil
}

// getMrc 获取逐字歌词
func (f *MgLyricFetcher) getMrc(urlStr string) (*struct{ Lyric, LxLyric string }, error) {
	text, err := f.getText(urlStr)
	if err != nil {
		return nil, err
	}

	// MRC 格式可能需要解密，这里先尝试直接解析
	// 如果是加密的，会返回空结果
	parsed := f.parseMrcLyric(text)
	return &parsed, nil
}

// getLrc 获取 LRC 歌词
func (f *MgLyricFetcher) getLrc(urlStr string) (*struct{ Lyric string }, error) {
	text, err := f.getText(urlStr)
	if err != nil {
		return nil, err
	}

	// 检查是否有时间标签
	lines := strings.Split(text, "\n")
	timeTagRe := regexp.MustCompile(`^\[(\d+):(\d+)\.(\d+)\]`)
	hasTimeTag := timeTagRe.MatchString(text)

	// 如果已经有时间标签，直接返回
	if hasTimeTag {
		linesWithTime := 0
		for _, line := range lines {
			if timeTagRe.MatchString(line) {
				linesWithTime++
			}
		}
		// 如果大部分行都有时间标签（>50%），认为是标准 LRC
		if linesWithTime > len(lines)/2 {
			return &struct{ Lyric string }{Lyric: text}, nil
		}
	}

	// 否则，为纯文本歌词添加假时间标签
	var lrcLines []string
	currentTime := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "@") {
			continue // 跳过空行和头部
		}

		// 为每行添加时间标签，每行间隔3秒
		minutes := currentTime / 60
		seconds := currentTime % 60
		timeTag := fmt.Sprintf("[%02d:%02d.00]", minutes, seconds)
		currentTime += 3

		lrcLines = append(lrcLines, timeTag+line)
	}

	return &struct{ Lyric string }{Lyric: strings.Join(lrcLines, "\n")}, nil
}

// parseMrcLyric 解析 MRC 格式歌词
func (f *MgLyricFetcher) parseMrcLyric(str string) struct{ Lyric, LxLyric string } {
	str = strings.ReplaceAll(str, "\r", "")
	lines := strings.Split(str, "\n")

	lineTimeRe := regexp.MustCompile(`^\s*\[(\d+),\d+\]`)
	wordTimeRe := regexp.MustCompile(`\(\d+,\d+\)`)
	wordTimeAllRe := regexp.MustCompile(`(\(\d+,\d+\))`)

	var lrcLines, lxlrcLines []string

	for _, line := range lines {
		if len(line) < 6 {
			continue
		}

		result := lineTimeRe.FindStringSubmatch(line)
		if result == nil {
			continue
		}

		startTime, _ := strconv.Atoi(result[1])
		time := startTime
		ms := time % 1000
		time /= 1000
		m := time / 60
		s := time % 60
		timeStr := fmt.Sprintf("%02d:%02d.%d", m, s, ms)

		words := lineTimeRe.ReplaceAllString(line, "")

		lrcLines = append(lrcLines, fmt.Sprintf("[%s]%s", timeStr, wordTimeAllRe.ReplaceAllString(words, "")))

		times := wordTimeAllRe.FindAllString(words, -1)
		if times == nil {
			continue
		}

		// 转换逐字时间标签
		var newTimes []string
		for _, t := range times {
			re := regexp.MustCompile(`\((\d+),(\d+)\)`)
			m := re.FindStringSubmatch(t)
			if m != nil {
				t1, _ := strconv.Atoi(m[1])
				t2, _ := strconv.Atoi(m[2])
				newTimes = append(newTimes, fmt.Sprintf("<%d,%d>", t1-startTime, t2))
			}
		}

		wordArr := wordTimeRe.Split(words, -1)
		var newWords strings.Builder
		for i, t := range newTimes {
			newWords.WriteString(t)
			if i < len(wordArr) {
				newWords.WriteString(wordArr[i])
			}
		}
		lxlrcLines = append(lxlrcLines, fmt.Sprintf("[%s]%s", timeStr, newWords.String()))
	}

	return struct{ Lyric, LxLyric string }{
		Lyric:   strings.Join(lrcLines, "\n"),
		LxLyric: strings.Join(lxlrcLines, "\n"),
	}
}
