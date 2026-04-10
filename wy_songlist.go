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

// WySongListProvider wy 平台歌单提供者
type WySongListProvider struct{}

// NewWySongListProvider 创建 wy 平台歌单提供者
func NewWySongListProvider() *WySongListProvider {
	return &WySongListProvider{}
}

// ID 返回平台标识
func (p *WySongListProvider) ID() string {
	return "wy"
}

// Name 返回平台名称
func (p *WySongListProvider) Name() string {
	return "wy"
}

// wy 歌单相关常量
const (
	wySongListLimit       = 30
	wySongListDetailLimit = 1000
	wySongListCookie      = "MUSIC_U="
)

// GetSortList 返回排序选项
func (p *WySongListProvider) GetSortList() []SortItem {
	return []SortItem{
		{ID: "hot", Name: "最热"},
	}
}

// GetTags 获取歌单标签
func (p *WySongListProvider) GetTags() (*TagResult, error) {
	// 获取分类标签
	tagParams, tagEncSecKey, err := weapiEncrypt(map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("wy getTags encrypt failed: %w", err)
	}

	tagFormData := fmt.Sprintf("params=%s&encSecKey=%s",
		url.QueryEscape(tagParams), url.QueryEscape(tagEncSecKey))

	tagBody, err := HTTPPostForm("https://music.163.com/weapi/playlist/catalogue", []byte(tagFormData), map[string]string{
		"User-Agent": wyUA,
		"Origin":     wyReferer,
		"Referer":    wyReferer,
	})
	if err != nil {
		return nil, fmt.Errorf("wy getTags request failed: %w", err)
	}

	var tagResp struct {
		Code       int `json:"code"`
		Categories map[string]string `json:"categories"`
		Sub        []struct {
			Category int    `json:"category"`
			Name     string `json:"name"`
		} `json:"sub"`
	}
	if err := json.Unmarshal(tagBody, &tagResp); err != nil {
		return nil, fmt.Errorf("wy getTags parse failed: %w", err)
	}
	if tagResp.Code != 200 {
		return nil, fmt.Errorf("wy getTags API error: code=%d", tagResp.Code)
	}

	// 按分类组织标签
	subList := make(map[int][]TagItem)
	for _, item := range tagResp.Sub {
		catKey := item.Category
		subList[catKey] = append(subList[catKey], TagItem{
			ID:     item.Name,
			Name:   item.Name,
			Parent: tagResp.Categories[fmt.Sprintf("%d", catKey)],
		})
	}

	var tagGroups []TagGroup
	for key, catName := range tagResp.Categories {
		catID, _ := strconv.Atoi(key)
		tagGroups = append(tagGroups, TagGroup{
			ID:   key,
			Name: catName,
			List: subList[catID],
		})
	}

	// 获取热门标签
	hotParams, hotEncSecKey, err := weapiEncrypt(map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("wy getHotTags encrypt failed: %w", err)
	}

	hotFormData := fmt.Sprintf("params=%s&encSecKey=%s",
		url.QueryEscape(hotParams), url.QueryEscape(hotEncSecKey))

	hotBody, err := HTTPPostForm("https://music.163.com/weapi/playlist/hottags", []byte(hotFormData), map[string]string{
		"User-Agent": wyUA,
		"Origin":     wyReferer,
		"Referer":    wyReferer,
	})

	var hotTags []TagItem
	if err == nil {
		var hotResp struct {
			Code int `json:"code"`
			Tags []struct {
				PlaylistTag struct {
					Name string `json:"name"`
				} `json:"playlistTag"`
			} `json:"tags"`
		}
		if json.Unmarshal(hotBody, &hotResp) == nil && hotResp.Code == 200 {
			for _, item := range hotResp.Tags {
				hotTags = append(hotTags, TagItem{
					ID:   item.PlaylistTag.Name,
					Name: item.PlaylistTag.Name,
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
func (p *WySongListProvider) GetList(sortId, tagId string, page int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}
	if sortId == "" {
		sortId = "hot"
	}
	if tagId == "" {
		tagId = "全部"
	}

	params, encSecKey, err := weapiEncrypt(map[string]interface{}{
		"cat":    tagId,
		"order":  sortId,
		"limit":  wySongListLimit,
		"offset": wySongListLimit * (page - 1),
		"total":  true,
	})
	if err != nil {
		return nil, fmt.Errorf("wy getList encrypt failed: %w", err)
	}

	formData := fmt.Sprintf("params=%s&encSecKey=%s",
		url.QueryEscape(params), url.QueryEscape(encSecKey))

	body, err := HTTPPostForm("https://music.163.com/weapi/playlist/list", []byte(formData), map[string]string{
		"User-Agent": wyUA,
		"Origin":     wyReferer,
		"Referer":    wyReferer,
	})
	if err != nil {
		return nil, fmt.Errorf("wy getList request failed: %w", err)
	}

	var resp struct {
		Code      int `json:"code"`
		Total     int `json:"total"`
		Playlists []struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			PlayCount   int    `json:"playCount"`
			CoverImgURL string `json:"coverImgUrl"`
			CreateTime  int64  `json:"createTime"`
			TrackCount  int    `json:"trackCount"`
			Description string `json:"description"`
			Grade       int    `json:"grade"`
			Creator     struct {
				Nickname string `json:"nickname"`
			} `json:"creator"`
		} `json:"playlists"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("wy getList parse failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("wy getList API error: code=%d", resp.Code)
	}

	var list []SongListItem
	for _, item := range resp.Playlists {
		timeStr := ""
		if item.CreateTime > 0 {
			timeStr = DateFormat(item.CreateTime)
		}
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(item.PlayCount),
			ID:        fmt.Sprintf("%d", item.ID),
			Author:    item.Creator.Nickname,
			Name:      item.Name,
			Time:      timeStr,
			Img:       item.CoverImgURL,
			Grade:     fmt.Sprintf("%d", item.Grade),
			Total:     fmt.Sprintf("%d", item.TrackCount),
			Desc:      item.Description,
		})
	}

	return &SongListResult{
		List:  list,
		Total: resp.Total,
		Page:  page,
		Limit: wySongListLimit,
	}, nil
}

// wyPlaylistTrack 歌单中的歌曲 track
type wyPlaylistTrack struct {
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
	Pc *struct {
		Ar  string `json:"ar"`
		Sn  string `json:"sn"`
		Alb string `json:"alb"`
	} `json:"pc"`
}

// wyPlaylistPrivilege 歌曲权限信息
type wyPlaylistPrivilege struct {
	ID         int64  `json:"id"`
	MaxBrLevel string `json:"maxBrLevel"`
	Maxbr      int    `json:"maxbr"`
}

// wyGetMusicDetail 歌曲详情批量获取，参考 netease/netease.go 的 fetchSongsBatch
func (p *WySongListProvider) wyGetMusicDetail(ids []int64) ([]SearchItem, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// 构造 c 参数
	type idItem struct {
		ID int64 `json:"id"`
	}
	cItems := make([]idItem, len(ids))
	for i, id := range ids {
		cItems[i] = idItem{ID: id}
	}
	cJSON, _ := json.Marshal(cItems)

	params, encSecKey, err := weapiEncrypt(map[string]interface{}{
		"c": string(cJSON),
	})
	if err != nil {
		return nil, fmt.Errorf("wy getMusicDetail encrypt failed: %w", err)
	}

	formData := fmt.Sprintf("params=%s&encSecKey=%s",
		url.QueryEscape(params), url.QueryEscape(encSecKey))

	body, err := HTTPPostForm("https://music.163.com/weapi/v3/song/detail", []byte(formData), map[string]string{
		"User-Agent": wyUA,
		"Origin":     wyReferer,
		"Referer":    wyReferer,
		"Cookie":     wySongListCookie,
	})
	if err != nil {
		return nil, fmt.Errorf("wy getMusicDetail request failed: %w", err)
	}

	var resp struct {
		Code       int                   `json:"code"`
		Songs      []wyPlaylistTrack     `json:"songs"`
		Privileges []wyPlaylistPrivilege `json:"privileges"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("wy getMusicDetail parse failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("wy getMusicDetail API error: code=%d", resp.Code)
	}

	return p.filterMusicDetailList(resp.Songs, resp.Privileges), nil
}

// filterMusicDetailList 转换歌曲详情列表为 SearchItem
func (p *WySongListProvider) filterMusicDetailList(tracks []wyPlaylistTrack, privileges []wyPlaylistPrivilege) []SearchItem {
	// 构建 privilege map
	privMap := make(map[int64]*wyPlaylistPrivilege)
	for i := range privileges {
		privMap[privileges[i].ID] = &privileges[i]
	}

	var list []SearchItem
	for i, track := range tracks {
		// 查找对应的 privilege
		var priv *wyPlaylistPrivilege
		if p, ok := privMap[track.ID]; ok {
			priv = p
		} else if i < len(privileges) {
			priv = &privileges[i]
		}
		if priv == nil {
			continue
		}

		item := p.convertTrackToSearchItem(&track, priv)
		list = append(list, item)
	}
	return list
}

// convertTrackToSearchItem 转换单个 track 为 SearchItem
func (p *WySongListProvider) convertTrackToSearchItem(track *wyPlaylistTrack, priv *wyPlaylistPrivilege) SearchItem {
	// 构建音质列表
	var types []QualityInfo

	if priv.MaxBrLevel == "hires" && track.Hr != nil {
		types = append(types, QualityInfo{
			Type: "flac24bit",
			Size: SizeToStr(track.Hr.Size),
		})
	}

	switch {
	case priv.Maxbr >= 999000:
		types = append(types, QualityInfo{Type: "flac"})
		fallthrough
	case priv.Maxbr >= 320000:
		size := ""
		if track.H != nil {
			size = SizeToStr(track.H.Size)
		}
		types = append(types, QualityInfo{Type: "320k", Size: size})
		fallthrough
	case priv.Maxbr >= 128000:
		size := ""
		if track.L != nil {
			size = SizeToStr(track.L.Size)
		}
		types = append(types, QualityInfo{Type: "128k", Size: size})
	}

	// 反转顺序（低品质在前）
	for i, j := 0, len(types)-1; i < j; i, j = i+1, j-1 {
		types[i], types[j] = types[j], types[i]
	}

	// 处理 pc（云盘歌曲）
	if track.Pc != nil {
		return SearchItem{
			Singer:  track.Pc.Ar,
			Name:    track.Pc.Sn,
			Album:   track.Pc.Alb,
			AlbumID: fmt.Sprintf("%d", track.Al.ID),
			Source:  "wy",
			Duration: int(track.Dt / 1000),
			MusicID: fmt.Sprintf("%d", track.ID),
			Songmid: fmt.Sprintf("%d", track.ID),
			Img:     track.Al.PicURL,
			Types:   types,
		}
	}

	// 拼接歌手名
	singerNames := make([]string, 0, len(track.Ar))
	for _, ar := range track.Ar {
		if ar.Name != "" {
			singerNames = append(singerNames, ar.Name)
		}
	}

	return SearchItem{
		Singer:  FormatSingers(singerNames),
		Name:    track.Name,
		Album:   track.Al.Name,
		AlbumID: fmt.Sprintf("%d", track.Al.ID),
		Source:  "wy",
		Duration: int(track.Dt / 1000),
		MusicID: fmt.Sprintf("%d", track.ID),
		Songmid: fmt.Sprintf("%d", track.ID),
		Img:     track.Al.PicURL,
		Types:   types,
	}
}

// filterListDetail 转换歌单详情中的 tracks + privileges
func (p *WySongListProvider) filterListDetail(tracks []wyPlaylistTrack, privileges []wyPlaylistPrivilege) []SearchItem {
	return p.filterMusicDetailList(tracks, privileges)
}

// handleParseId 处理 URL 重定向跟随
func (p *WySongListProvider) handleParseId(link string) (string, error) {
	listDetailLinkRe := regexp.MustCompile(`[?&]id=(\d+)`)
	listDetailLink2Re := regexp.MustCompile(`/playlist/(\d+)/\d+/.+$`)

	body, err := HTTPGet(link, nil)
	if err != nil {
		return "", fmt.Errorf("wy handleParseId request failed: %w", err)
	}

	bodyStr := string(body)

	matches := listDetailLinkRe.FindStringSubmatch(bodyStr)
	if matches != nil && len(matches) > 1 {
		return matches[1], nil
	}

	matches2 := listDetailLink2Re.FindStringSubmatch(bodyStr)
	if matches2 != nil && len(matches2) > 1 {
		return matches2[1], nil
	}

	// 尝试从链接本身提取
	matches = listDetailLinkRe.FindStringSubmatch(link)
	if matches != nil && len(matches) > 1 {
		return matches[1], nil
	}

	matches2 = listDetailLink2Re.FindStringSubmatch(link)
	if matches2 != nil && len(matches2) > 1 {
		return matches2[1], nil
	}

	return "", fmt.Errorf("wy handleParseId: cannot extract id")
}

// getListId 从各种格式的 ID 中提取纯数字 ID
func (p *WySongListProvider) getListId(rawId string) (string, string, error) {
	cookie := wySongListCookie

	// 处理 ### 分隔的 cookie
	if strings.Contains(rawId, "###") {
		parts := strings.SplitN(rawId, "###", 2)
		rawId = parts[0]
		if len(parts) > 1 {
			cookie = "MUSIC_U=" + parts[1]
		}
	}

	listDetailLinkRe := regexp.MustCompile(`[?&]id=(\d+)`)
	listDetailLink2Re := regexp.MustCompile(`/playlist/(\d+)/\d+/.+$`)

	if strings.Contains(rawId, "?") || strings.Contains(rawId, "&") || strings.Contains(rawId, ":") || strings.Contains(rawId, "/") {
		matches := listDetailLinkRe.FindStringSubmatch(rawId)
		if matches != nil && len(matches) > 1 {
			return matches[1], cookie, nil
		}

		matches2 := listDetailLink2Re.FindStringSubmatch(rawId)
		if matches2 != nil && len(matches2) > 1 {
			return matches2[1], cookie, nil
		}

		// 尝试重定向跟随
		parsedID, err := p.handleParseId(rawId)
		if err != nil {
			return "", cookie, err
		}
		return parsedID, cookie, nil
	}

	return rawId, cookie, nil
}

// GetListDetail 获取歌单详情
func (p *WySongListProvider) GetListDetail(rawId string, page int) (*SongListDetailResult, error) {
	if page < 1 {
		page = 1
	}

	slog.Info("wy getListDetail", "id", rawId, "page", page)

	id, cookie, err := p.getListId(rawId)
	if err != nil {
		return nil, fmt.Errorf("wy getListDetail parse id failed: %w", err)
	}

	// 使用 linuxapi 获取歌单详情
	encryptedParams, err := linuxapiEncrypt(map[string]interface{}{
		"method": "POST",
		"url":    "https://music.163.com/api/v3/playlist/detail",
		"params": map[string]interface{}{
			"id": id,
			"n":  100000,
			"s":  8,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("wy getListDetail encrypt failed: %w", err)
	}

	formData := "eparams=" + encryptedParams

	body, err := HTTPPostForm("https://music.163.com/api/linux/forward", []byte(formData), map[string]string{
		"User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.3112.90 Safari/537.36",
		"Cookie":     cookie,
	})
	if err != nil {
		return nil, fmt.Errorf("wy getListDetail request failed: %w", err)
	}

	var resp struct {
		Code     int `json:"code"`
		Playlist struct {
			Name        string `json:"name"`
			CoverImgURL string `json:"coverImgUrl"`
			Description string `json:"description"`
			PlayCount   int    `json:"playCount"`
			Creator     struct {
				Nickname string `json:"nickname"`
			} `json:"creator"`
			TrackIDs []struct {
				ID int64 `json:"id"`
			} `json:"trackIds"`
			Tracks []wyPlaylistTrack `json:"tracks"`
		} `json:"playlist"`
		Privileges []wyPlaylistPrivilege `json:"privileges"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("wy getListDetail parse failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("wy getListDetail API error: code=%d", resp.Code)
	}

	totalTracks := len(resp.Playlist.TrackIDs)

	// 判断是否需要批量获取歌曲详情
	var songs []SearchItem
	if len(resp.Playlist.TrackIDs) == len(resp.Privileges) {
		// tracks 和 privileges 数量一致，直接转换
		songs = p.filterListDetail(resp.Playlist.Tracks, resp.Privileges)
	} else {
		// 需要批量获取歌曲详情
		rangeStart := (page - 1) * wySongListDetailLimit
		rangeEnd := page * wySongListDetailLimit
		if rangeEnd > totalTracks {
			rangeEnd = totalTracks
		}
		if rangeStart >= totalTracks {
			songs = []SearchItem{}
		} else {
			// 提取当前页的 trackIds
			pageTrackIDs := make([]int64, 0, rangeEnd-rangeStart)
			for i := rangeStart; i < rangeEnd; i++ {
				pageTrackIDs = append(pageTrackIDs, resp.Playlist.TrackIDs[i].ID)
			}

			// 批量获取歌曲详情
			detailSongs, err := p.wyGetMusicDetail(pageTrackIDs)
			if err != nil {
				return nil, fmt.Errorf("wy getListDetail batch detail failed: %w", err)
			}
			songs = detailSongs
		}
	}

	return &SongListDetailResult{
		List:  songs,
		Total: totalTracks,
		Page:  page,
		Limit: wySongListDetailLimit,
		Info: &SongListInfo{
			Name:      resp.Playlist.Name,
			Img:       resp.Playlist.CoverImgURL,
			Desc:      resp.Playlist.Description,
			Author:    resp.Playlist.Creator.Nickname,
			PlayCount: FormatPlayCount(resp.Playlist.PlayCount),
		},
	}, nil
}

// SearchSongList 搜索歌单
func (p *WySongListProvider) SearchSongList(keyword string, page int, limit int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	slog.Info("wy searchSongList", "keyword", keyword, "page", page)

	// 使用 eapiEncrypt 搜索歌单
	searchParams := map[string]interface{}{
		"s":      keyword,
		"type":   1000, // 1000 = 歌单
		"limit":  limit,
		"total":  page == 1,
		"offset": limit * (page - 1),
	}

	encryptedParams, err := eapiEncrypt("/api/cloudsearch/pc", searchParams)
	if err != nil {
		return nil, fmt.Errorf("wy searchSongList encrypt failed: %w", err)
	}

	formData := "params=" + encryptedParams

	body, err := HTTPPostForm("https://interface3.music.163.com/eapi/batch", []byte(formData), map[string]string{
		"User-Agent": wyUA,
		"Origin":     wyReferer,
	})
	if err != nil {
		return nil, fmt.Errorf("wy searchSongList request failed: %w", err)
	}

	var resp struct {
		Code   int `json:"code"`
		Result struct {
			PlaylistCount int `json:"playlistCount"`
			Playlists     []struct {
				ID          int64  `json:"id"`
				Name        string `json:"name"`
				PlayCount   int    `json:"playCount"`
				CoverImgURL string `json:"coverImgUrl"`
				CreateTime  int64  `json:"createTime"`
				TrackCount  int    `json:"trackCount"`
				Description string `json:"description"`
				Grade       int    `json:"grade"`
				Creator     struct {
					Nickname string `json:"nickname"`
				} `json:"creator"`
			} `json:"playlists"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("wy searchSongList parse failed: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("wy searchSongList API error: code=%d", resp.Code)
	}

	var list []SongListItem
	for _, item := range resp.Result.Playlists {
		timeStr := ""
		if item.CreateTime > 0 {
			timeStr = DateFormat(item.CreateTime)
		}
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(item.PlayCount),
			ID:        fmt.Sprintf("%d", item.ID),
			Author:    item.Creator.Nickname,
			Name:      item.Name,
			Time:      timeStr,
			Img:       item.CoverImgURL,
			Grade:     fmt.Sprintf("%d", item.Grade),
			Total:     fmt.Sprintf("%d", item.TrackCount),
			Desc:      item.Description,
		})
	}

	return &SongListResult{
		List:  list,
		Total: resp.Result.PlaylistCount,
		Page:  page,
		Limit: limit,
	}, nil
}
