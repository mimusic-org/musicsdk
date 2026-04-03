//go:build wasip1

package musicsdk

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 咪咕音乐签名常量
const (
	mgSignatureMd5    = "6cdc72a439cef99a3418d2a78aa28c73"
	mgSecretKey       = "yyapp2d16148780a1dcc7408e06336b98cfd50"
	mgDefaultDeviceId = "963B7AA0D21511ED807EE5846EC87D20"
)

// MgSearcher 咪咕音乐搜索器
type MgSearcher struct {
	deviceId string
}

// NewMgSearcher 创建咪咕搜索器
func NewMgSearcher() *MgSearcher {
	return &MgSearcher{
		deviceId: mgDefaultDeviceId,
	}
}

// ID 返回搜索器标识
func (s *MgSearcher) ID() string {
	return "mg"
}

// Name 返回搜索器名称
func (s *MgSearcher) Name() string {
	return "咪咕音乐"
}

// mgSearchResponse 咪咕搜索 API 响应
type mgSearchResponse struct {
	Code           string           `json:"code"`
	Info           string           `json:"info"`
	SongResultData mgSongResultData `json:"songResultData"`
}

// mgSongResultData 歌曲搜索结果数据
type mgSongResultData struct {
	TotalCount string           `json:"totalCount"`
	ResultList [][]mgSearchItem `json:"resultList"` // 二维数组
}

// mgSearchItem 咪咕搜索结果项
type mgSearchItem struct {
	Name         string          `json:"name"`
	SongId       string          `json:"songId"`
	CopyrightId  string          `json:"copyrightId"`
	Album        string          `json:"album"`
	AlbumId      string          `json:"albumId"`
	Duration     interface{}     `json:"duration"` // 可能是数字或字符串 "MM:SS"
	Img1         string          `json:"img1"`
	Img2         string          `json:"img2"`
	Img3         string          `json:"img3"`
	SingerList   []mgSinger      `json:"singerList"`
	AudioFormats []mgAudioFormat `json:"audioFormats"`
	LrcUrl       string          `json:"lrcUrl"`
	MrcUrl       string          `json:"mrcurl"`
	TrcUrl       string          `json:"trcUrl"`
}

// mgSinger 咪咕歌手信息
type mgSinger struct {
	Name string `json:"name"`
}

// mgAudioFormat 咪咕音频格式
type mgAudioFormat struct {
	FormatType string `json:"formatType"` // PQ, HQ, SQ, ZQ24
	ASize      int64  `json:"asize"`      // Android size
	ISize      int64  `json:"isize"`      // iOS size
}

// mgSign 生成咪咕签名
func mgSign(keyword string, deviceId string) (sign string, timestamp string) {
	timestamp = strconv.FormatInt(time.Now().UnixMilli(), 10)
	// 签名格式: MD5(keyword + signatureMd5 + secretKey + deviceId + timestamp)
	data := keyword + mgSignatureMd5 + mgSecretKey + deviceId + timestamp
	hash := md5.Sum([]byte(data))
	sign = hex.EncodeToString(hash[:])
	return sign, timestamp
}

// Search 搜索歌曲
func (s *MgSearcher) Search(keyword string, page int, limit int) (*SearchResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}

	// 生成签名
	sign, timestamp := mgSign(keyword, s.deviceId)

	// 构建请求 URL
	searchSwitch := `{"song":1,"album":0,"singer":0,"tagSong":1,"mvSong":0,"bestShow":1,"songlist":0,"lyricSong":0}`

	params := url.Values{}
	params.Set("isCorrect", "0")
	params.Set("isCopyright", "1")
	params.Set("searchSwitch", searchSwitch)
	params.Set("pageSize", fmt.Sprintf("%d", limit))
	params.Set("text", keyword)
	params.Set("pageNo", fmt.Sprintf("%d", page))
	params.Set("sort", "0")
	params.Set("sid", "USS")

	apiURL := "https://jadeite.migu.cn/music_search/v3/search/searchAll?" + params.Encode()

	slog.Info("mg search", "keyword", keyword, "page", page, "url", apiURL)

	// 设置请求头
	headers := map[string]string{
		"uiVersion":  "A_music_3.6.1",
		"deviceId":   s.deviceId,
		"timestamp":  timestamp,
		"sign":       sign,
		"channel":    "0146921",
		"User-Agent": "Mozilla/5.0 (Linux; U; Android 11.0.0; zh-cn; MI 11 Build/OPR1.170623.032) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30",
	}

	// 发送请求
	body, err := HTTPGet(apiURL, headers)
	if err != nil {
		return nil, fmt.Errorf("mg search request failed: %w", err)
	}

	slog.Info("mg search response", "respLen", len(body))

	// 解析响应
	var resp mgSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("mg search parse response failed: %w", err)
	}

	// 检查响应状态
	if resp.Code != "000000" {
		return nil, fmt.Errorf("mg search API error: code=%s, info=%s", resp.Code, resp.Info)
	}

	// 转换数据
	list := s.filterData(resp.SongResultData.ResultList)

	// 解析总数
	total, _ := strconv.Atoi(resp.SongResultData.TotalCount)

	slog.Info("mg search result", "total", total, "items", len(list))

	return &SearchResult{
		List:  list,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// filterData 处理搜索结果
func (s *MgSearcher) filterData(resultList [][]mgSearchItem) []SearchItem {
	seen := make(map[string]bool)
	var list []SearchItem

	for _, items := range resultList {
		for _, item := range items {
			// 去重：使用 copyrightId
			if item.SongId == "" || item.CopyrightId == "" || seen[item.CopyrightId] {
				continue
			}
			seen[item.CopyrightId] = true

			list = append(list, s.convertItem(item))
		}
	}

	return list
}

// convertItem 转换单个搜索项
func (s *MgSearcher) convertItem(item mgSearchItem) SearchItem {
	// 处理歌手名
	singerNames := make([]string, 0, len(item.SingerList))
	for _, singer := range item.SingerList {
		if singer.Name != "" {
			singerNames = append(singerNames, singer.Name)
		}
	}
	singer := DecodeName(FormatSingers(singerNames))

	// 处理封面图
	img := item.Img3
	if img == "" {
		img = item.Img2
	}
	if img == "" {
		img = item.Img1
	}
	if img != "" && !strings.HasPrefix(img, "http") {
		img = "http://d.musicapp.migu.cn" + img
	}

	// 处理时长
	duration := s.parseDuration(item.Duration)

	// 处理音质列表
	types := s.parseAudioFormats(item.AudioFormats)

	return SearchItem{
		Name:        DecodeName(item.Name),
		Singer:      singer,
		Album:       DecodeName(item.Album),
		AlbumID:     item.AlbumId,
		Duration:    duration,
		Source:      "mg",
		MusicID:     item.SongId,
		Img:         img,
		CopyrightId: item.CopyrightId,
		Types:       types,
	}
}

// parseDuration 解析时长（支持秒数或 "MM:SS" 格式）
func (s *MgSearcher) parseDuration(d interface{}) int {
	if d == nil {
		return 0
	}

	switch v := d.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		// 尝试解析 "MM:SS" 格式
		parts := strings.Split(v, ":")
		if len(parts) == 2 {
			minutes, _ := strconv.Atoi(parts[0])
			seconds, _ := strconv.Atoi(parts[1])
			return minutes*60 + seconds
		}
		// 尝试直接解析为数字
		if num, err := strconv.Atoi(v); err == nil {
			return num
		}
	}
	return 0
}

// parseAudioFormats 解析音频格式列表
func (s *MgSearcher) parseAudioFormats(formats []mgAudioFormat) []QualityInfo {
	var types []QualityInfo

	for _, format := range formats {
		size := format.ASize
		if size == 0 {
			size = format.ISize
		}

		var qualityType string
		switch format.FormatType {
		case "PQ":
			qualityType = "128k"
		case "HQ":
			qualityType = "320k"
		case "SQ":
			qualityType = "flac"
		case "ZQ", "ZQ24":
			qualityType = "flac24bit"
		default:
			continue
		}

		types = append(types, QualityInfo{
			Type: qualityType,
			Size: SizeToStr(size),
		})
	}

	return types
}
