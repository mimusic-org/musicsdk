//go:build wasip1

package musicsdk

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
)

// KgSearcher kg 平台搜索器
type KgSearcher struct{}

// NewKgSearcher 创建 kg 平台搜索器
func NewKgSearcher() *KgSearcher {
	return &KgSearcher{}
}

// ID 返回搜索器标识
func (s *KgSearcher) ID() string {
	return "kg"
}

// Name 返回搜索器名称
func (s *KgSearcher) Name() string {
	return "kg"
}

// kgSearchResponse kg 搜索 API 响应
type kgSearchResponse struct {
	ErrorCode int `json:"error_code"`
	Data      struct {
		Total int            `json:"total"`
		Lists []kgSearchItem `json:"lists"`
	} `json:"data"`
}

// kgSearchItem kg 搜索结果项
type kgSearchItem struct {
	SongName    string     `json:"SongName"`
	Singers     []kgSinger `json:"Singers"`
	AlbumName   string     `json:"AlbumName"`
	AlbumID     string     `json:"AlbumID"`
	Duration    int        `json:"Duration"`
	Image       string     `json:"Image"`
	FileHash    string     `json:"FileHash"`
	FileSize    int64      `json:"FileSize"`
	HQFileHash  string     `json:"HQFileHash"`
	HQFileSize  int64      `json:"HQFileSize"`
	SQFileHash  string     `json:"SQFileHash"`
	SQFileSize  int64      `json:"SQFileSize"`
	ResFileHash string     `json:"ResFileHash"`
	ResFileSize int64      `json:"ResFileSize"`
	Audioid     int        `json:"Audioid"`
	TransParam  struct {
		UnionCover string `json:"union_cover"`
	} `json:"trans_param"`
	Grp []kgSearchItem `json:"Grp"`
}

// kgSinger kg 歌手信息
type kgSinger struct {
	Name string `json:"name"`
}

// Search 搜索歌曲
func (s *KgSearcher) Search(keyword string, page int, limit int) (*SearchResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}

	// 构建请求 URL
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("page", fmt.Sprintf("%d", page))
	params.Set("pagesize", fmt.Sprintf("%d", limit))
	params.Set("platform", "WebFilter")
	params.Set("filter", "2")
	params.Set("iscorrection", "1")
	params.Set("privilege_filter", "0")

	apiURL := "https://songsearch.kugou.com/song_search_v2?" + params.Encode()

	slog.Info("kg search", "keyword", keyword, "page", page, "url", apiURL)

	// 发送请求
	body, err := HTTPGet(apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kg search request failed: %w", err)
	}

	slog.Info("kg search response", "respLen", len(body))

	// 解析响应
	var resp kgSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kg search parse response failed: %w", err)
	}

	// 检查响应状态
	if resp.ErrorCode != 0 {
		return nil, fmt.Errorf("kg search API error: error_code=%d", resp.ErrorCode)
	}

	// 转换数据
	list := s.handleResult(resp.Data.Lists)

	slog.Info("kg search result", "total", resp.Data.Total, "items", len(list))

	return &SearchResult{
		List:  list,
		Total: resp.Data.Total,
		Page:  page,
		Limit: limit,
	}, nil
}

// handleResult 处理搜索结果，去重并展开 Grp
func (s *KgSearcher) handleResult(items []kgSearchItem) []SearchItem {
	seen := make(map[string]bool)
	var list []SearchItem

	for _, item := range items {
		key := fmt.Sprintf("%d_%s", item.Audioid, item.FileHash)
		if !seen[key] {
			seen[key] = true
			list = append(list, s.filterData(item))
		}

		// 处理 Grp 中的子项
		for _, grpItem := range item.Grp {
			grpKey := fmt.Sprintf("%d_%s", grpItem.Audioid, grpItem.FileHash)
			if !seen[grpKey] {
				seen[grpKey] = true
				list = append(list, s.filterData(grpItem))
			}
		}
	}

	return list
}

// filterData 转换单个搜索项
func (s *KgSearcher) filterData(item kgSearchItem) SearchItem {
	// 处理歌手名
	singerNames := make([]string, 0, len(item.Singers))
	for _, singer := range item.Singers {
		if singer.Name != "" {
			singerNames = append(singerNames, singer.Name)
		}
	}
	singer := DecodeName(FormatSingers(singerNames))

	// 处理封面图
	img := item.Image
	if img != "" {
		img = strings.ReplaceAll(img, "{size}", "240")
	} else if item.TransParam.UnionCover != "" {
		img = strings.ReplaceAll(item.TransParam.UnionCover, "{size}", "240")
	}

	// 处理音质列表
	var types []QualityInfo

	// 128k
	if item.FileHash != "" && item.FileSize != 0 {
		types = append(types, QualityInfo{
			Type: "128k",
			Size: SizeToStr(item.FileSize),
			Hash: item.FileHash,
		})
	}

	// 320k
	if item.HQFileHash != "" && item.HQFileSize != 0 {
		types = append(types, QualityInfo{
			Type: "320k",
			Size: SizeToStr(item.HQFileSize),
			Hash: item.HQFileHash,
		})
	}

	// flac
	if item.SQFileHash != "" && item.SQFileSize != 0 {
		types = append(types, QualityInfo{
			Type: "flac",
			Size: SizeToStr(item.SQFileSize),
			Hash: item.SQFileHash,
		})
	}

	// flac24bit
	if item.ResFileHash != "" && item.ResFileSize != 0 {
		types = append(types, QualityInfo{
			Type: "flac24bit",
			Size: SizeToStr(item.ResFileSize),
			Hash: item.ResFileHash,
		})
	}

	return SearchItem{
		Name:     DecodeName(item.SongName),
		Singer:   singer,
		Album:    DecodeName(item.AlbumName),
		AlbumID:  item.AlbumID,
		Duration: item.Duration,
		Source:   "kg",
		MusicID:  item.FileHash,
		Img:      img,
		Hash:     item.FileHash,
		Types:    types,
	}
}
