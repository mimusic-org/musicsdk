//go:build wasip1

package musicsdk

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

const (
	wyAPIURL   = "https://interface3.music.163.com/eapi/batch"
	wyUA       = "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36"
	wyReferer  = "https://music.163.com"
	wySourceID = "wy"
)

// WySearcher wy 平台搜索器
type WySearcher struct{}

// NewWySearcher 创建 wy 平台搜索器
func NewWySearcher() *WySearcher {
	return &WySearcher{}
}

// ID 返回平台 ID
func (s *WySearcher) ID() string {
	return wySourceID
}

// Name 返回平台名称
func (s *WySearcher) Name() string {
	return "wy"
}

// wySearchParams 搜索参数（与 lxserver musicSearch.js 保持一致）
type wySearchParams struct {
	Keyword     string `json:"keyword"`
	NeedCorrect string `json:"needCorrect"`
	Channel     string `json:"channel"`
	Offset      int    `json:"offset"`
	Scene       string `json:"scene"`
	Total       bool   `json:"total"`
	Limit       int    `json:"limit"`
}

// wySearchData 搜索响应
type wySearchData struct {
	Code int `json:"code"`
	Data struct {
		TotalCount int              `json:"totalCount"`
		Resources  []wyResourceItem `json:"resources"`
	} `json:"data"`
}

type wyResourceItem struct {
	BaseInfo struct {
		SimpleSongData wySongData `json:"simpleSongData"`
	} `json:"baseInfo"`
	Privilege struct {
		MaxBrLevel string `json:"maxBrLevel"`
		Maxbr      int    `json:"maxbr"`
	} `json:"privilege"`
}

type wySongData struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Dt   int64  `json:"dt"` // 毫秒
	Ar   []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"ar"`
	Al struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		PicURL string `json:"picUrl"`
	} `json:"al"`
	Hr *wyQualitySize `json:"hr"`
	Sq *wyQualitySize `json:"sq"`
	H  *wyQualitySize `json:"h"`
	L  *wyQualitySize `json:"l"`
}

type wyQualitySize struct {
	Size int64 `json:"size"`
}

// Search 执行搜索
func (s *WySearcher) Search(keyword string, page int, limit int) (*SearchResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}

	// 构造搜索参数（与 lxserver musicSearch.js 保持一致）
	searchParams := wySearchParams{
		Keyword:     keyword,
		NeedCorrect: "1",
		Channel:     "typing",
		Offset:      (page - 1) * limit,
		Scene:       "normal",
		Total:       page == 1,
		Limit:       limit,
	}

	// eapi 加密：直接传搜索参数
	encryptedParams, err := eapiEncrypt("/api/search/song/list/page", searchParams)
	if err != nil {
		return nil, fmt.Errorf("encrypt params: %w", err)
	}

	// 构造表单数据
	formData := "params=" + encryptedParams

	slog.Info("wy search", "keyword", keyword, "page", page, "url", wyAPIURL)

	// 发送请求（与 lxserver 保持一致：Origin 头而非 Referer）
	headers := map[string]string{
		"User-Agent": wyUA,
		"Origin":     wyReferer,
	}
	respBytes, err := HTTPPostForm(wyAPIURL, []byte(formData), headers)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	slog.Info("wy search response", "respLen", len(respBytes))

	// 解析响应 - 直接传参时响应格式: { "code": 200, "data": { "totalCount": N, "resources": [...] } }
	var searchResp wySearchData
	if err := json.Unmarshal(respBytes, &searchResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if searchResp.Code != 200 {
		return nil, fmt.Errorf("api error: code=%d", searchResp.Code)
	}

	// 转换结果
	items := make([]SearchItem, 0, len(searchResp.Data.Resources))
	for _, resource := range searchResp.Data.Resources {
		item := s.convertResourceItem(&resource)
		items = append(items, item)
	}

	slog.Info("wy search result", "total", searchResp.Data.TotalCount, "items", len(items))

	return &SearchResult{
		List:  items,
		Total: searchResp.Data.TotalCount,
		Page:  page,
		Limit: limit,
	}, nil
}

// convertResourceItem 转换资源项
func (s *WySearcher) convertResourceItem(resource *wyResourceItem) SearchItem {
	song := &resource.BaseInfo.SimpleSongData

	// 拼接歌手名
	singerNames := make([]string, 0, len(song.Ar))
	for _, ar := range song.Ar {
		singerNames = append(singerNames, ar.Name)
	}

	// 构建音质列表
	types := make([]QualityInfo, 0, 4)
	privilege := resource.Privilege

	// 根据 maxbr 和 maxBrLevel 判断可用音质
	if song.L != nil && song.L.Size > 0 {
		types = append(types, QualityInfo{
			Type: "128k",
			Size: SizeToStr(song.L.Size),
		})
	}
	if song.H != nil && song.H.Size > 0 && privilege.Maxbr >= 320000 {
		types = append(types, QualityInfo{
			Type: "320k",
			Size: SizeToStr(song.H.Size),
		})
	}
	if song.Sq != nil && song.Sq.Size > 0 && privilege.Maxbr >= 999000 {
		types = append(types, QualityInfo{
			Type: "flac",
			Size: SizeToStr(song.Sq.Size),
		})
	}
	if song.Hr != nil && song.Hr.Size > 0 && privilege.MaxBrLevel == "hires" {
		types = append(types, QualityInfo{
			Type: "flac24bit",
			Size: SizeToStr(song.Hr.Size),
		})
	}

	return SearchItem{
		Name:     DecodeName(song.Name),
		Singer:   FormatSingers(singerNames),
		Album:    DecodeName(song.Al.Name),
		AlbumID:  fmt.Sprintf("%d", song.Al.ID),
		Duration: int(song.Dt / 1000), // 毫秒转秒
		Source:   wySourceID,
		MusicID:  fmt.Sprintf("%d", song.ID),
		Img:      song.Al.PicURL,
		Types:    types,
		Songmid:  fmt.Sprintf("%d", song.ID),
	}
}
