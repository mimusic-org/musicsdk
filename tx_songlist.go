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

// TxSongListProvider tx 平台歌单提供者
type TxSongListProvider struct{}

// NewTxSongListProvider 创建 tx 平台歌单提供者
func NewTxSongListProvider() *TxSongListProvider {
	return &TxSongListProvider{}
}

// ID 返回平台标识
func (p *TxSongListProvider) ID() string {
	return "tx"
}

// Name 返回平台名称
func (p *TxSongListProvider) Name() string {
	return "tx"
}

// tx 歌单相关常量
const (
	txSongListLimit = 36
)

// GetSortList 返回排序选项
func (p *TxSongListProvider) GetSortList() []SortItem {
	return []SortItem{
		{ID: "5", Name: "最热"},
		{ID: "2", Name: "最新"},
	}
}

// GetTags 获取歌单标签
func (p *TxSongListProvider) GetTags() (*TagResult, error) {
	// 获取分类标签
	tagsURL := "https://u.y.qq.com/cgi-bin/musicu.fcg?loginUin=0&hostUin=0&format=json&inCharset=utf-8&outCharset=utf-8&notice=0&platform=wk_v15.json&needNewCode=0&data=%7B%22tags%22%3A%7B%22method%22%3A%22get_all_categories%22%2C%22param%22%3A%7B%22qq%22%3A%22%22%7D%2C%22module%22%3A%22playlist.PlaylistAllCategoriesServer%22%7D%7D"

	tagBody, err := HTTPGet(tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("tx getTags request failed: %w", err)
	}

	var tagResp struct {
		Code int `json:"code"`
		Tags struct {
			Code int `json:"code"`
			Data struct {
				VGroup []struct {
					GroupID   int    `json:"group_id"`
					GroupName string `json:"group_name"`
					VItem     []struct {
						ID   int    `json:"id"`
						Name string `json:"name"`
					} `json:"v_item"`
				} `json:"v_group"`
			} `json:"data"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(tagBody, &tagResp); err != nil {
		return nil, fmt.Errorf("tx getTags parse failed: %w", err)
	}
	if tagResp.Code != 0 {
		return nil, fmt.Errorf("tx getTags API error: code=%d", tagResp.Code)
	}

	var tagGroups []TagGroup
	for _, group := range tagResp.Tags.Data.VGroup {
		var items []TagItem
		for _, item := range group.VItem {
			items = append(items, TagItem{
				ID:     fmt.Sprintf("%d", item.ID),
				Name:   item.Name,
				Parent: fmt.Sprintf("%d", group.GroupID),
			})
		}
		tagGroups = append(tagGroups, TagGroup{
			ID:   fmt.Sprintf("%d", group.GroupID),
			Name: group.GroupName,
			List: items,
		})
	}

	// 获取热门标签（HTML 解析）
	var hotTags []TagItem
	hotTagURL := "https://c.y.qq.com/node/pc/wk_v15/category_playlist.html"
	hotBody, hotErr := HTTPGet(hotTagURL, nil)
	if hotErr == nil {
		hotTagHTMLRe := regexp.MustCompile(`class="c_bg_link js_tag_item" data-id="\w+">.+?</a>`)
		hotTagRe := regexp.MustCompile(`data-id="(\w+)">(.+?)</a>`)
		matches := hotTagHTMLRe.FindAllString(string(hotBody), -1)
		for _, match := range matches {
			result := hotTagRe.FindStringSubmatch(match)
			if result != nil && len(result) > 2 {
				hotTags = append(hotTags, TagItem{
					ID:   result[1],
					Name: result[2],
				})
			}
		}
	}

	return &TagResult{
		Tags: tagGroups,
		Hot:  hotTags,
	}, nil
}

// GetList 获取歌单列表
func (p *TxSongListProvider) GetList(sortId, tagId string, page int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}
	if sortId == "" {
		sortId = "5"
	}

	var apiURL string
	if tagId != "" {
		// 按标签获取
		tagIDInt, _ := strconv.Atoi(tagId)
		reqData := map[string]interface{}{
			"comm": map[string]interface{}{"cv": 1602, "ct": 20},
			"playlist": map[string]interface{}{
				"method": "get_category_content",
				"param": map[string]interface{}{
					"titleid":     tagIDInt,
					"caller":      "0",
					"category_id": tagIDInt,
					"size":        txSongListLimit,
					"page":        page - 1,
					"use_page":    1,
				},
				"module": "playlist.PlayListCategoryServer",
			},
		}
		reqJSON, _ := json.Marshal(reqData)
		apiURL = fmt.Sprintf("https://u.y.qq.com/cgi-bin/musicu.fcg?loginUin=0&hostUin=0&format=json&inCharset=utf-8&outCharset=utf-8&notice=0&platform=wk_v15.json&needNewCode=0&data=%s",
			url.QueryEscape(string(reqJSON)))
	} else {
		// 推荐列表
		sortIDInt, _ := strconv.Atoi(sortId)
		reqData := map[string]interface{}{
			"comm": map[string]interface{}{"cv": 1602, "ct": 20},
			"playlist": map[string]interface{}{
				"method": "get_playlist_by_tag",
				"param": map[string]interface{}{
					"id":       10000000,
					"sin":      txSongListLimit * (page - 1),
					"size":     txSongListLimit,
					"order":    sortIDInt,
					"cur_page": page,
				},
				"module": "playlist.PlayListPlazaServer",
			},
		}
		reqJSON, _ := json.Marshal(reqData)
		apiURL = fmt.Sprintf("https://u.y.qq.com/cgi-bin/musicu.fcg?loginUin=0&hostUin=0&format=json&inCharset=utf-8&outCharset=utf-8&notice=0&platform=wk_v15.json&needNewCode=0&data=%s",
			url.QueryEscape(string(reqJSON)))
	}

	body, err := HTTPGet(apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("tx getList request failed: %w", err)
	}

	var resp struct {
		Code     int `json:"code"`
		Playlist struct {
			Code int `json:"code"`
			Data json.RawMessage `json:"data"`
		} `json:"playlist"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("tx getList parse failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("tx getList API error: code=%d", resp.Code)
	}

	if tagId != "" {
		return p.filterList2(resp.Playlist.Data, page)
	}
	return p.filterList(resp.Playlist.Data, page)
}

// filterList 转换推荐歌单列表
func (p *TxSongListProvider) filterList(data json.RawMessage, page int) (*SongListResult, error) {
	var listData struct {
		Total      int `json:"total"`
		VPlaylist  []struct {
			Tid         int    `json:"tid"`
			Title       string `json:"title"`
			Desc        string `json:"desc"`
			AccessNum   int    `json:"access_num"`
			CoverURLMed string `json:"cover_url_medium"`
			ModifyTime  int64  `json:"modify_time"`
			SongIDs     []int  `json:"song_ids"`
			CreatorInfo struct {
				Nick string `json:"nick"`
			} `json:"creator_info"`
		} `json:"v_playlist"`
	}
	if err := json.Unmarshal(data, &listData); err != nil {
		return nil, fmt.Errorf("tx filterList parse failed: %w", err)
	}

	var list []SongListItem
	for _, item := range listData.VPlaylist {
		timeStr := ""
		if item.ModifyTime > 0 {
			timeStr = DateFormat(item.ModifyTime)
		}
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(item.AccessNum),
			ID:        fmt.Sprintf("%d", item.Tid),
			Author:    item.CreatorInfo.Nick,
			Name:      item.Title,
			Time:      timeStr,
			Img:       item.CoverURLMed,
			Total:     fmt.Sprintf("%d", len(item.SongIDs)),
			Desc:      DecodeName(strings.ReplaceAll(item.Desc, "<br>", "\n")),
		})
	}

	return &SongListResult{
		List:  list,
		Total: listData.Total,
		Page:  page,
		Limit: txSongListLimit,
	}, nil
}

// filterList2 转换按标签歌单列表
func (p *TxSongListProvider) filterList2(data json.RawMessage, page int) (*SongListResult, error) {
	var listData struct {
		Content struct {
			TotalCnt int `json:"total_cnt"`
			VItem    []struct {
				Basic struct {
					Tid      int    `json:"tid"`
					Title    string `json:"title"`
					Desc     string `json:"desc"`
					PlayCnt  int    `json:"play_cnt"`
					Creator  struct {
						Nick string `json:"nick"`
					} `json:"creator"`
					Cover struct {
						MediumURL  string `json:"medium_url"`
						DefaultURL string `json:"default_url"`
					} `json:"cover"`
				} `json:"basic"`
			} `json:"v_item"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &listData); err != nil {
		return nil, fmt.Errorf("tx filterList2 parse failed: %w", err)
	}

	var list []SongListItem
	for _, item := range listData.Content.VItem {
		img := item.Basic.Cover.MediumURL
		if img == "" {
			img = item.Basic.Cover.DefaultURL
		}
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(item.Basic.PlayCnt),
			ID:        fmt.Sprintf("%d", item.Basic.Tid),
			Author:    item.Basic.Creator.Nick,
			Name:      item.Basic.Title,
			Img:       img,
			Desc:      DecodeName(strings.ReplaceAll(item.Basic.Desc, "<br>", "\n")),
		})
	}

	return &SongListResult{
		List:  list,
		Total: listData.Content.TotalCnt,
		Page:  page,
		Limit: txSongListLimit,
	}, nil
}

// txListDetailSongItem tx 歌单详情中的歌曲项
type txListDetailSongItem struct {
	Name     string `json:"title"`
	Mid      string `json:"mid"`
	ID       int64  `json:"id"`
	Interval int    `json:"interval"`
	Singer   []struct {
		Name string `json:"name"`
		Mid  string `json:"mid"`
	} `json:"singer"`
	Album struct {
		Name string `json:"name"`
		Mid  string `json:"mid"`
	} `json:"album"`
	File struct {
		MediaMid   string `json:"media_mid"`
		Size128mp3 int64  `json:"size_128mp3"`
		Size320mp3 int64  `json:"size_320mp3"`
		SizeFlac   int64  `json:"size_flac"`
		SizeHires  int64  `json:"size_hires"`
	} `json:"file"`
}

// filterListDetail 转换歌单详情中的歌曲数据
func (p *TxSongListProvider) filterListDetail(items []txListDetailSongItem) []SearchItem {
	var list []SearchItem
	for _, item := range items {
		// 拼接歌手名
		singerNames := make([]string, 0, len(item.Singer))
		for _, singer := range item.Singer {
			if singer.Name != "" {
				singerNames = append(singerNames, singer.Name)
			}
		}

		// 生成封面 URL
		var img string
		albumMid := item.Album.Mid
		if albumMid != "" && item.Album.Name != "" && item.Album.Name != "空" {
			img = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R500x500M000%s.jpg", albumMid)
		} else if len(item.Singer) > 0 && item.Singer[0].Mid != "" {
			img = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T001R500x500M000%s.jpg", item.Singer[0].Mid)
		}

		// 构建音质列表
		var types []QualityInfo
		if item.File.Size128mp3 != 0 {
			types = append(types, QualityInfo{
				Type: "128k",
				Size: SizeToStr(item.File.Size128mp3),
			})
		}
		if item.File.Size320mp3 != 0 {
			types = append(types, QualityInfo{
				Type: "320k",
				Size: SizeToStr(item.File.Size320mp3),
			})
		}
		if item.File.SizeFlac != 0 {
			types = append(types, QualityInfo{
				Type: "flac",
				Size: SizeToStr(item.File.SizeFlac),
			})
		}
		if item.File.SizeHires != 0 {
			types = append(types, QualityInfo{
				Type: "flac24bit",
				Size: SizeToStr(item.File.SizeHires),
			})
		}

		list = append(list, SearchItem{
			Singer:      FormatSingers(singerNames),
			Name:        item.Name,
			Album:       item.Album.Name,
			AlbumID:     item.Album.Mid,
			Source:      "tx",
			Duration:    item.Interval,
			MusicID:     item.Mid,
			Songmid:     item.Mid,
			AlbumMid:    albumMid,
			StrMediaMid: item.File.MediaMid,
			Img:         img,
			Types:       types,
		})
	}
	return list
}

// handleParseId 处理 URL 重定向跟随
func (p *TxSongListProvider) handleParseId(link string) (string, error) {
	// 在 WASM 环境中，直接请求链接获取重定向
	body, err := HTTPGet(link, nil)
	if err != nil {
		return "", fmt.Errorf("tx handleParseId request failed: %w", err)
	}

	// 尝试从响应体中提取 ID
	bodyStr := string(body)
	listDetailLinkRe := regexp.MustCompile(`/playlist/(\d+)`)
	matches := listDetailLinkRe.FindStringSubmatch(bodyStr)
	if matches != nil && len(matches) > 1 {
		return matches[1], nil
	}

	listDetailLink2Re := regexp.MustCompile(`id=(\d+)`)
	matches2 := listDetailLink2Re.FindStringSubmatch(bodyStr)
	if matches2 != nil && len(matches2) > 1 {
		return matches2[1], nil
	}

	return "", fmt.Errorf("tx handleParseId: cannot extract id from link")
}

// getListId 从各种格式的 ID 中提取纯数字 ID
func (p *TxSongListProvider) getListId(id string) (string, error) {
	listDetailLinkRe := regexp.MustCompile(`/playlist/(\d+)`)
	listDetailLink2Re := regexp.MustCompile(`id=(\d+)`)

	if strings.Contains(id, "?") || strings.Contains(id, "&") || strings.Contains(id, ":") || strings.Contains(id, "/") {
		matches := listDetailLinkRe.FindStringSubmatch(id)
		if matches != nil && len(matches) > 1 {
			return matches[1], nil
		}

		matches2 := listDetailLink2Re.FindStringSubmatch(id)
		if matches2 != nil && len(matches2) > 1 {
			return matches2[1], nil
		}

		// 尝试重定向跟随
		parsedID, err := p.handleParseId(id)
		if err != nil {
			return "", err
		}
		return parsedID, nil
	}

	return id, nil
}

// GetListDetail 获取歌单详情
func (p *TxSongListProvider) GetListDetail(id string, page int) (*SongListDetailResult, error) {
	slog.Info("tx getListDetail", "id", id, "page", page)

	parsedID, err := p.getListId(id)
	if err != nil {
		return nil, fmt.Errorf("tx getListDetail parse id failed: %w", err)
	}

	apiURL := fmt.Sprintf("https://c.y.qq.com/qzone/fcg-bin/fcg_ucc_getcdinfo_byids_cp.fcg?type=1&json=1&utf8=1&onlysong=0&new_format=1&disstid=%s&loginUin=0&hostUin=0&format=json&inCharset=utf8&outCharset=utf-8&notice=0&platform=yqq.json&needNewCode=0",
		parsedID)

	body, err := HTTPGet(apiURL, map[string]string{
		"Origin":  "https://y.qq.com",
		"Referer": fmt.Sprintf("https://y.qq.com/n/yqq/playsquare/%s.html", parsedID),
	})
	if err != nil {
		return nil, fmt.Errorf("tx getListDetail request failed: %w", err)
	}

	var resp struct {
		Code   int `json:"code"`
		CDList []struct {
			DissName string                 `json:"dissname"`
			Logo     string                 `json:"logo"`
			Desc     string                 `json:"desc"`
			Nickname string                 `json:"nickname"`
			VisitNum int                    `json:"visitnum"`
			SongList []txListDetailSongItem `json:"songlist"`
		} `json:"cdlist"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("tx getListDetail parse failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("tx getListDetail API error: code=%d", resp.Code)
	}
	if len(resp.CDList) == 0 {
		return nil, fmt.Errorf("tx getListDetail: empty cdlist")
	}

	cdlist := resp.CDList[0]
	list := p.filterListDetail(cdlist.SongList)

	return &SongListDetailResult{
		List:  list,
		Total: len(cdlist.SongList),
		Page:  1,
		Limit: len(cdlist.SongList) + 1,
		Info: &SongListInfo{
			Name:      cdlist.DissName,
			Img:       cdlist.Logo,
			Desc:      DecodeName(strings.ReplaceAll(cdlist.Desc, "<br>", "\n")),
			Author:    cdlist.Nickname,
			PlayCount: FormatPlayCount(cdlist.VisitNum),
		},
	}, nil
}

// SearchSongList 搜索歌单
func (p *TxSongListProvider) SearchSongList(keyword string, page int, limit int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	apiURL := fmt.Sprintf("http://c.y.qq.com/soso/fcgi-bin/client_music_search_songlist?page_no=%d&num_per_page=%d&format=json&query=%s&remoteplace=txt.yqq.playlist&inCharset=utf8&outCharset=utf-8",
		page-1, limit, url.QueryEscape(keyword))

	slog.Info("tx searchSongList", "keyword", keyword, "page", page, "url", apiURL)

	body, err := HTTPGet(apiURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (compatible; MSIE 9.0; Windows NT 6.1; WOW64; Trident/5.0)",
		"Referer":    "http://y.qq.com/portal/search.html",
	})
	if err != nil {
		return nil, fmt.Errorf("tx searchSongList request failed: %w", err)
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Sum  int `json:"sum"`
			List []struct {
				DissID       string `json:"dissid"`
				DissName     string `json:"dissname"`
				ImgURL       string `json:"imgurl"`
				ListenNum    int    `json:"listennum"`
				SongCount    int    `json:"song_count"`
				CreateTime   string `json:"createtime"`
				Introduction string `json:"introduction"`
				Creator      struct {
					Name string `json:"name"`
				} `json:"creator"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("tx searchSongList parse failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("tx searchSongList API error: code=%d", resp.Code)
	}

	var list []SongListItem
	for _, item := range resp.Data.List {
		timeStr := ""
		if item.CreateTime != "" {
			if ct, err := strconv.ParseInt(item.CreateTime, 10, 64); err == nil && ct > 0 {
				timeStr = DateFormat(ct)
			}
		}
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(item.ListenNum),
			ID:        item.DissID,
			Author:    DecodeName(item.Creator.Name),
			Name:      DecodeName(item.DissName),
			Time:      timeStr,
			Img:       item.ImgURL,
			Total:     fmt.Sprintf("%d", item.SongCount),
			Desc:      DecodeName(strings.ReplaceAll(DecodeName(item.Introduction), "<br>", "\n")),
		})
	}

	return &SongListResult{
		List:  list,
		Total: resp.Data.Sum,
		Page:  page,
		Limit: limit,
	}, nil
}
