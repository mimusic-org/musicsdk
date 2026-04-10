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

// MgSongListProvider mg 平台歌单提供者
type MgSongListProvider struct {
	deviceId string
}

// NewMgSongListProvider 创建 mg 平台歌单提供者
func NewMgSongListProvider() *MgSongListProvider {
	return &MgSongListProvider{
		deviceId: mgDefaultDeviceId,
	}
}

// ID 返回平台标识
func (p *MgSongListProvider) ID() string {
	return "mg"
}

// Name 返回平台名称
func (p *MgSongListProvider) Name() string {
	return "mg"
}

// mg 歌单相关常量
const (
	mgSongListLimit       = 30
	mgSongListDetailLimit = 50
)

// mg 默认请求头
var mgSongListHeaders = map[string]string{
	"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1",
	"Referer":    "https://m.music.migu.cn/",
}

// GetSortList 返回排序选项
func (p *MgSongListProvider) GetSortList() []SortItem {
	return []SortItem{
		{ID: "15127315", Name: "推荐"},
	}
}

// GetTags 获取歌单标签
func (p *MgSongListProvider) GetTags() (*TagResult, error) {
	apiURL := "https://app.c.nf.migu.cn/pc/v1.0/template/musiclistplaza-taglist/release"

	body, err := HTTPGet(apiURL, mgSongListHeaders)
	if err != nil {
		return nil, fmt.Errorf("mg getTags request failed: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Data []struct {
			Header struct {
				Title string `json:"title"`
			} `json:"header"`
			Content []struct {
				Texts []string `json:"texts"` // [name, id]
			} `json:"content"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("mg getTags parse failed: %w", err)
	}
	if resp.Code != "000000" {
		return nil, fmt.Errorf("mg getTags API error: code=%s", resp.Code)
	}

	if len(resp.Data) == 0 {
		return &TagResult{}, nil
	}

	// 第一个元素是热门标签
	var hotTags []TagItem
	if len(resp.Data) > 0 {
		for _, item := range resp.Data[0].Content {
			if len(item.Texts) >= 2 {
				hotTags = append(hotTags, TagItem{
					ID:   item.Texts[1],
					Name: item.Texts[0],
				})
			}
		}
	}

	// 后续元素是分组标签
	var tagGroups []TagGroup
	for _, group := range resp.Data[1:] {
		var items []TagItem
		for _, item := range group.Content {
			if len(item.Texts) >= 2 {
				items = append(items, TagItem{
					ID:   item.Texts[1],
					Name: item.Texts[0],
				})
			}
		}
		tagGroups = append(tagGroups, TagGroup{
			Name: group.Header.Title,
			List: items,
		})
	}

	return &TagResult{
		Tags: tagGroups,
		Hot:  hotTags,
	}, nil
}

// GetList 获取歌单列表
func (p *MgSongListProvider) GetList(sortId, tagId string, page int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}

	var apiURL string
	if tagId == "" {
		// 推荐列表
		apiURL = fmt.Sprintf("https://app.c.nf.migu.cn/pc/bmw/page-data/playlist-square-recommend/v1.0?templateVersion=2&pageNo=%d", page)
	} else {
		// 按标签获取
		apiURL = fmt.Sprintf("https://app.c.nf.migu.cn/pc/v1.0/template/musiclistplaza-listbytag/release?pageNumber=%d&templateVersion=2&tagId=%s", page, tagId)
	}

	body, err := HTTPGet(apiURL, mgSongListHeaders)
	if err != nil {
		return nil, fmt.Errorf("mg getList request failed: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Data struct {
			Contents        json.RawMessage `json:"contents"`
			ContentItemList json.RawMessage `json:"contentItemList"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("mg getList parse failed: %w", err)
	}
	if resp.Code != "000000" {
		return nil, fmt.Errorf("mg getList API error: code=%s", resp.Code)
	}

	var list []SongListItem

	// 尝试解析 contents 格式（推荐列表）
	if resp.Data.Contents != nil && string(resp.Data.Contents) != "null" {
		list = p.filterListFromContents(resp.Data.Contents)
	} else if resp.Data.ContentItemList != nil && string(resp.Data.ContentItemList) != "null" {
		// 尝试解析 contentItemList 格式（按标签）
		list = p.filterListFromContentItemList(resp.Data.ContentItemList)
	}

	return &SongListResult{
		List:  list,
		Total: 99999,
		Page:  page,
		Limit: mgSongListLimit,
	}, nil
}

// mgContentItem 递归内容项
type mgContentItem struct {
	ResType  string          `json:"resType"`
	ResID    string          `json:"resId"`
	Txt      string          `json:"txt"`
	Txt2     string          `json:"txt2"`
	Img      string          `json:"img"`
	Contents []mgContentItem `json:"contents"`
}

// filterListFromContents 从 contents 格式中提取歌单列表（递归）
func (p *MgSongListProvider) filterListFromContents(data json.RawMessage) []SongListItem {
	var contents []mgContentItem
	if err := json.Unmarshal(data, &contents); err != nil {
		return nil
	}

	ids := make(map[string]bool)
	var list []SongListItem
	p.extractSongListItems(contents, &list, ids)
	return list
}

// extractSongListItems 递归提取歌单项
func (p *MgSongListProvider) extractSongListItems(contents []mgContentItem, list *[]SongListItem, ids map[string]bool) {
	for _, item := range contents {
		if len(item.Contents) > 0 {
			p.extractSongListItems(item.Contents, list, ids)
		} else if item.ResType == "2021" && !ids[item.ResID] {
			ids[item.ResID] = true
			*list = append(*list, SongListItem{
				ID:   item.ResID,
				Name: item.Txt,
				Img:  item.Img,
				Desc: item.Txt2,
			})
		}
	}
}

// filterListFromContentItemList 从 contentItemList 格式中提取歌单列表
func (p *MgSongListProvider) filterListFromContentItemList(data json.RawMessage) []SongListItem {
	var contentItemList []struct {
		ItemList []struct {
			Title    string `json:"title"`
			ImageURL string `json:"imageUrl"`
			LogEvent struct {
				ContentID string `json:"contentId"`
			} `json:"logEvent"`
			BarList []struct {
				Title string `json:"title"`
			} `json:"barList"`
		} `json:"itemList"`
	}
	if err := json.Unmarshal(data, &contentItemList); err != nil {
		return nil
	}

	var list []SongListItem
	// 使用第二个 contentItem（索引 1）
	if len(contentItemList) > 1 {
		for _, item := range contentItemList[1].ItemList {
			playCount := ""
			if len(item.BarList) > 0 {
				playCount = item.BarList[0].Title
			}
			list = append(list, SongListItem{
				PlayCount: playCount,
				ID:        item.LogEvent.ContentID,
				Name:      item.Title,
				Img:       item.ImageURL,
			})
		}
	}

	return list
}

// mgSongListDetailSong 歌单详情中的歌曲项
type mgSongListDetailSong struct {
	SongID      string `json:"songId"`
	SongName    string `json:"songName"`
	CopyrightID string `json:"copyrightId"`
	Album       string `json:"album"`
	AlbumID     string `json:"albumId"`
	AlbumImgs   []struct {
		ImgSizeType string `json:"imgSizeType"`
		Img         string `json:"img"`
	} `json:"albumImgs"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	LengthInSecond interface{} `json:"lengthInSecond"`
	AudioFormats   []struct {
		FormatType string `json:"formatType"`
		ASize      string `json:"asize"`
		ISize      string `json:"isize"`
	} `json:"audioFormats"`
}

// filterMusicInfoListV5 转换歌单详情歌曲数据为 SearchItem，参考 migu/migu.go 的 convertItemToSong
func (p *MgSongListProvider) filterMusicInfoListV5(songs []mgSongListDetailSong) []SearchItem {
	var list []SearchItem
	for _, song := range songs {
		if song.SongID == "" {
			continue
		}

		// 处理歌手名
		singerNames := make([]string, 0, len(song.Artists))
		for _, artist := range song.Artists {
			if artist.Name != "" {
				singerNames = append(singerNames, artist.Name)
			}
		}

		// 处理封面图
		img := ""
		for _, albumImg := range song.AlbumImgs {
			if albumImg.Img != "" {
				img = albumImg.Img
				break
			}
		}
		if img != "" && !strings.HasPrefix(img, "http") {
			img = "http://d.musicapp.migu.cn" + img
		}

		// 处理时长
		duration := 0
		switch v := song.LengthInSecond.(type) {
		case float64:
			duration = int(v)
		case string:
			duration, _ = strconv.Atoi(v)
		}

		// 处理音质
		var types []QualityInfo
		for _, format := range song.AudioFormats {
			aSize, _ := strconv.ParseInt(format.ASize, 10, 64)
			iSize, _ := strconv.ParseInt(format.ISize, 10, 64)
			size := aSize
			if size == 0 {
				size = iSize
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

		list = append(list, SearchItem{
			Singer:      FormatSingers(singerNames),
			Name:        DecodeName(song.SongName),
			Album:       DecodeName(song.Album),
			AlbumID:     song.AlbumID,
			Source:      "mg",
			Duration:    duration,
			MusicID:     song.SongID,
			CopyrightId: song.CopyrightID,
			Img:         img,
			Types:       types,
		})
	}
	return list
}

// getListDetailList 获取歌单歌曲列表
func (p *MgSongListProvider) getListDetailList(id string, page int) ([]SearchItem, int, error) {
	apiURL := fmt.Sprintf("https://app.c.nf.migu.cn/MIGUM3.0/resource/playlist/song/v2.0?pageNo=%d&pageSize=%d&playlistId=%s",
		page, mgSongListDetailLimit, id)

	body, err := HTTPGet(apiURL, mgSongListHeaders)
	if err != nil {
		return nil, 0, fmt.Errorf("mg getListDetailList request failed: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Data struct {
			TotalCount int                     `json:"totalCount"`
			SongList   []mgSongListDetailSong  `json:"songList"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("mg getListDetailList parse failed: %w", err)
	}
	if resp.Code != "000000" {
		return nil, 0, fmt.Errorf("mg getListDetailList API error: code=%s", resp.Code)
	}

	list := p.filterMusicInfoListV5(resp.Data.SongList)
	return list, resp.Data.TotalCount, nil
}

// getListDetailInfo 获取歌单信息
func (p *MgSongListProvider) getListDetailInfo(id string) (*SongListInfo, error) {
	apiURL := fmt.Sprintf("https://c.musicapp.migu.cn/MIGUM3.0/resource/playlist/v2.0?playlistId=%s", id)

	body, err := HTTPGet(apiURL, mgSongListHeaders)
	if err != nil {
		return nil, fmt.Errorf("mg getListDetailInfo request failed: %w", err)
	}

	var resp struct {
		Code string `json:"code"`
		Data struct {
			Title     string `json:"title"`
			Summary   string `json:"summary"`
			OwnerName string `json:"ownerName"`
			ImgItem   struct {
				Img string `json:"img"`
			} `json:"imgItem"`
			OpNumItem struct {
				PlayNum int `json:"playNum"`
			} `json:"opNumItem"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("mg getListDetailInfo parse failed: %w", err)
	}
	if resp.Code != "000000" {
		return nil, fmt.Errorf("mg getListDetailInfo API error: code=%s", resp.Code)
	}

	return &SongListInfo{
		Name:      resp.Data.Title,
		Img:       resp.Data.ImgItem.Img,
		Desc:      resp.Data.Summary,
		Author:    resp.Data.OwnerName,
		PlayCount: FormatPlayCount(resp.Data.OpNumItem.PlayNum),
	}, nil
}

// GetListDetail 获取歌单详情
func (p *MgSongListProvider) GetListDetail(id string, page int) (*SongListDetailResult, error) {
	if page < 1 {
		page = 1
	}

	slog.Info("mg getListDetail", "id", id, "page", page)

	// 处理各种 URL 格式
	listDetailLinkRe := regexp.MustCompile(`/playlist/(\d+)`)
	playlistParamRe := regexp.MustCompile(`(?:playlistId|id)=(\d+)`)

	if strings.Contains(id, "/playlist") {
		if strings.Contains(id, "playlist/index.html") || strings.Contains(id, "/playlist?") {
			matches := playlistParamRe.FindStringSubmatch(id)
			if matches != nil && len(matches) > 1 {
				id = matches[1]
			}
		} else {
			matches := listDetailLinkRe.FindStringSubmatch(id)
			if matches != nil && len(matches) > 1 {
				id = matches[1]
			}
		}
	} else if strings.Contains(id, "?") || strings.Contains(id, "&") || strings.Contains(id, ":") || strings.Contains(id, "/") {
		// 尝试从 URL 中提取 playlistId
		matches := playlistParamRe.FindStringSubmatch(id)
		if matches != nil && len(matches) > 1 {
			id = matches[1]
		} else {
			// 尝试请求获取重定向
			redirectBody, err := HTTPGet(id, map[string]string{
				"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46",
				"Referer":    id,
			})
			if err == nil {
				bodyStr := string(redirectBody)
				matches = playlistParamRe.FindStringSubmatch(bodyStr)
				if matches != nil && len(matches) > 1 {
					id = matches[1]
				} else {
					matches = listDetailLinkRe.FindStringSubmatch(bodyStr)
					if matches != nil && len(matches) > 1 {
						id = matches[1]
					}
				}
			}
		}
	}

	// 并行获取歌曲列表和歌单信息
	songs, total, listErr := p.getListDetailList(id, page)
	info, infoErr := p.getListDetailInfo(id)

	if listErr != nil {
		return nil, listErr
	}

	if infoErr != nil {
		slog.Warn("mg getListDetailInfo failed", "id", id, "error", infoErr)
		info = &SongListInfo{}
	}

	return &SongListDetailResult{
		List:  songs,
		Total: total,
		Page:  page,
		Limit: mgSongListDetailLimit,
		Info:  info,
	}, nil
}

// SearchSongList 搜索歌单
func (p *MgSongListProvider) SearchSongList(keyword string, page int, limit int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	slog.Info("mg searchSongList", "keyword", keyword, "page", page)

	// 复用 mg_search.go 的 mgSign 签名
	sign, timestamp := mgSign(keyword, p.deviceId)

	searchSwitch := `{"song":0,"album":0,"singer":0,"tagSong":0,"mvSong":0,"bestShow":0,"songlist":1,"lyricSong":0}`

	apiURL := fmt.Sprintf("https://jadeite.migu.cn/music_search/v3/search/searchAll?isCorrect=1&isCopyright=1&searchSwitch=%s&pageSize=%d&text=%s&pageNo=%d&sort=0&sid=USS",
		url.QueryEscape(searchSwitch), limit, url.QueryEscape(keyword), page)

	body, err := HTTPGet(apiURL, map[string]string{
		"uiVersion":  "A_music_3.6.1",
		"deviceId":   p.deviceId,
		"timestamp":  timestamp,
		"sign":       sign,
		"channel":    "0146921",
		"User-Agent": "Mozilla/5.0 (Linux; U; Android 11.0.0; zh-cn; MI 11 Build/OPR1.170623.032) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30",
	})
	if err != nil {
		return nil, fmt.Errorf("mg searchSongList request failed: %w", err)
	}

	var resp struct {
		SongListResultData *struct {
			TotalCount string `json:"totalCount"`
			Result     []struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				UserName     string `json:"userName"`
				MusicListPicURL string `json:"musicListPicUrl"`
				MusicNum     string `json:"musicNum"`
				PlayNum      string `json:"playNum"`
			} `json:"result"`
		} `json:"songListResultData"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("mg searchSongList parse failed: %w", err)
	}
	if resp.SongListResultData == nil {
		return nil, fmt.Errorf("mg searchSongList: no songListResultData")
	}

	total, _ := strconv.Atoi(resp.SongListResultData.TotalCount)

	var list []SongListItem
	for _, item := range resp.SongListResultData.Result {
		if item.ID == "" {
			continue
		}
		playNum, _ := strconv.Atoi(item.PlayNum)
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(playNum),
			ID:        item.ID,
			Author:    item.UserName,
			Name:      item.Name,
			Img:       item.MusicListPicURL,
			Total:     item.MusicNum,
		})
	}

	return &SongListResult{
		List:  list,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}
