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

// KwSearcher kw 平台搜索器
type KwSearcher struct{}

// NewKwSearcher 创建 kw 平台搜索器
func NewKwSearcher() *KwSearcher {
	return &KwSearcher{}
}

// ID 返回搜索器标识
func (s *KwSearcher) ID() string {
	return "kw"
}

// Name 返回搜索器名称
func (s *KwSearcher) Name() string {
	return "kw"
}

// kwSearchResponse kw 搜索 API 响应
type kwSearchResponse struct {
	Total   string         `json:"TOTAL"`
	Show    string         `json:"SHOW"`
	Abslist []kwSearchItem `json:"abslist"`
}

// kwSearchItem kw 搜索结果项
type kwSearchItem struct {
	SongName         string `json:"SONGNAME"`
	Artist           string `json:"ARTIST"`
	Album            string `json:"ALBUM"`
	AlbumID          string `json:"ALBUMID"`
	MusicRid         string `json:"MUSICRID"`
	Duration         string `json:"DURATION"`
	NMinfo           string `json:"N_MINFO"`
	WebAlbumpicShort string `json:"web_albumpic_short"`
	HtsMvpic         string `json:"hts_MVPIC"`
	ProbAlbumpic     string `json:"prob_albumpic"`
}

// Search 搜索歌曲
func (s *KwSearcher) Search(keyword string, page int, limit int) (*SearchResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}

	// 构建请求 URL（酷我 page 从 0 开始）
	// 必须携带 rformat=json&encoding=utf8 才能返回 JSON 格式
	params := url.Values{}
	params.Set("client", "kt")
	params.Set("all", keyword)
	params.Set("pn", fmt.Sprintf("%d", page-1))
	params.Set("rn", fmt.Sprintf("%d", limit))
	params.Set("uid", "794762570")
	params.Set("ver", "kwplayer_ar_9.2.2.1")
	params.Set("vipver", "1")
	params.Set("show_copyright_off", "1")
	params.Set("newver", "1")
	params.Set("ft", "music")
	params.Set("cluster", "0")
	params.Set("strategy", "2012")
	params.Set("encoding", "utf8")
	params.Set("rformat", "json")
	params.Set("vermerge", "1")
	params.Set("mobi", "1")
	params.Set("issubtitle", "1")

	apiURL := "http://search.kuwo.cn/r.s?" + params.Encode()

	slog.Info("kw search", "keyword", keyword, "page", page, "url", apiURL)

	// 发送请求
	body, err := HTTPGet(apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kw search request failed: %w", err)
	}

	slog.Info("kw search response", "respLen", len(body))

	// 酷我返回的可能是类 JSON 格式，需要预处理
	jsonStr := s.fixJsonFormat(string(body))

	// 解析响应
	var resp kwSearchResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("kw search parse response failed: %w", err)
	}

	// 检查响应状态
	if resp.Total == "0" || resp.Total == "" {
		return &SearchResult{
			List:  []SearchItem{},
			Total: 0,
			Page:  page,
			Limit: limit,
		}, nil
	}

	// 转换数据
	list := s.handleResult(resp.Abslist)

	total, _ := strconv.Atoi(resp.Total)

	slog.Info("kw search result", "total", total, "items", len(list))

	return &SearchResult{
		List:  list,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// fixJsonFormat 修复 kw 返回的非标准 JSON 格式
// kw 有时返回单引号 JSON，需要转换为双引号
// Go regexp 不支持 lookbehind，改用逐字符状态机实现
func (s *KwSearcher) fixJsonFormat(str string) string {
	var buf strings.Builder
	buf.Grow(len(str))

	for i := 0; i < len(str); i++ {
		ch := str[i]
		if ch != '\'' {
			buf.WriteByte(ch)
			continue
		}
		// 判断单引号前一个非空白字符
		prevIdx := i - 1
		for prevIdx >= 0 && (str[prevIdx] == ' ' || str[prevIdx] == '\t') {
			prevIdx--
		}
		var prev byte
		if prevIdx >= 0 {
			prev = str[prevIdx]
		}
		// 判断单引号后一个非空白字符
		nextIdx := i + 1
		for nextIdx < len(str) && (str[nextIdx] == ' ' || str[nextIdx] == '\t') {
			nextIdx++
		}
		var next byte
		if nextIdx < len(str) {
			next = str[nextIdx]
		}
		// 符合 JSON 结构边界时替换为双引号
		if prev == ':' || prev == ',' || prev == '[' || prev == '{' ||
			next == ':' || next == ',' || next == ']' || next == '}' || next == '\'' {
			buf.WriteByte('"')
		} else {
			buf.WriteByte(ch)
		}
	}

	return buf.String()
}

// handleResult 处理搜索结果
func (s *KwSearcher) handleResult(items []kwSearchItem) []SearchItem {
	var list []SearchItem

	for _, item := range items {
		searchItem := s.filterData(item)
		if searchItem.MusicID != "" {
			list = append(list, searchItem)
		}
	}

	return list
}

// filterData 转换单个搜索项
func (s *KwSearcher) filterData(item kwSearchItem) SearchItem {
	// 处理歌曲 ID（去掉 MUSIC_ 前缀）
	musicID := strings.TrimPrefix(item.MusicRid, "MUSIC_")

	// 处理歌手名（& 替换为 、）
	singer := DecodeName(strings.ReplaceAll(item.Artist, "&", "、"))

	// 处理时长
	duration, _ := strconv.Atoi(item.Duration)

	// 处理封面图
	img := s.getAlbumPic(item)

	// 解析音质信息
	types := s.parseMInfo(item.NMinfo)

	return SearchItem{
		Name:     DecodeName(item.SongName),
		Singer:   singer,
		Album:    DecodeName(item.Album),
		AlbumID:  DecodeName(item.AlbumID),
		Duration: duration,
		Source:   "kw",
		MusicID:  musicID,
		Songmid:  musicID, // kw 的 songmid 即为 musicID
		Img:      img,
		Types:    types,
	}
}

// getAlbumPic 获取专辑封面
func (s *KwSearcher) getAlbumPic(item kwSearchItem) string {
	// 优先使用 prob_albumpic
	if item.ProbAlbumpic != "" {
		return item.ProbAlbumpic
	}

	// 其次使用 web_albumpic_short 拼接完整路径
	if item.WebAlbumpicShort != "" {
		return "https://img4.kuwo.cn/star/albumcover/500" + item.WebAlbumpicShort
	}

	// 最后使用 hts_mvpic
	if item.HtsMvpic != "" {
		return item.HtsMvpic
	}

	return ""
}

// parseMInfo 解析 N_MINFO 字段获取音质信息
// 格式: level:xx,bitrate:xx,format:xx,size:xx;level:...
func (s *KwSearcher) parseMInfo(minfo string) []QualityInfo {
	if minfo == "" {
		return nil
	}

	// 正则匹配单个音质信息
	re := regexp.MustCompile(`level:(\w+),bitrate:(\d+),format:(\w+),size:([\w.]+)`)

	var types []QualityInfo
	typeMap := make(map[string]bool) // 防止重复

	// 按分号分隔多个音质
	parts := strings.Split(minfo, ";")
	for _, part := range parts {
		matches := re.FindStringSubmatch(part)
		if len(matches) < 5 {
			continue
		}

		bitrate := matches[2]
		format := matches[3]
		size := strings.ToUpper(matches[4])

		var qualityType string
		switch bitrate {
		case "4000":
			qualityType = "flac24bit"
		case "2000":
			qualityType = "flac"
		case "320":
			qualityType = "320k"
		case "128":
			qualityType = "128k"
		default:
			// 如果 bitrate 不匹配，检查 format
			if format == "flac" {
				qualityType = "flac"
			} else {
				continue
			}
		}

		// 避免重复
		if typeMap[qualityType] {
			continue
		}
		typeMap[qualityType] = true

		types = append(types, QualityInfo{
			Type: qualityType,
			Size: size,
		})
	}

	// 反转顺序（低品质在前）
	for i, j := 0, len(types)-1; i < j; i, j = i+1, j-1 {
		types[i], types[j] = types[j], types[i]
	}

	return types
}
