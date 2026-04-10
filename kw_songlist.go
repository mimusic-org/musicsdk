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

// KwSongListProvider kw 平台歌单提供者
type KwSongListProvider struct{}

// NewKwSongListProvider 创建 kw 平台歌单提供者
func NewKwSongListProvider() *KwSongListProvider {
	return &KwSongListProvider{}
}

// ID 返回平台标识
func (p *KwSongListProvider) ID() string {
	return "kw"
}

// Name 返回平台名称
func (p *KwSongListProvider) Name() string {
	return "kw"
}

// kw 歌单相关常量
const (
	kwSongListLimit       = 36
	kwSongListDetailLimit = 1000
)

// GetSortList 返回排序选项
func (p *KwSongListProvider) GetSortList() []SortItem {
	return []SortItem{
		{ID: "new", Name: "最新"},
		{ID: "hot", Name: "最热"},
	}
}

// GetTags 获取歌单标签
func (p *KwSongListProvider) GetTags() (*TagResult, error) {
	tagsURL := "http://wapi.kuwo.cn/api/pc/classify/playlist/getTagList?cmd=rcm_keyword_playlist&user=0&prod=kwplayer_pc_9.0.5.0&vipver=9.0.5.0&source=kwplayer_pc_9.0.5.0&loginUid=0&loginSid=0&appUid=76039576"
	hotTagURL := "http://wapi.kuwo.cn/api/pc/classify/playlist/getRcmTagList?loginUid=0&loginSid=0&appUid=76039576"

	// 并行获取标签和热门标签
	tagBody, tagErr := HTTPGet(tagsURL, nil)
	hotBody, hotErr := HTTPGet(hotTagURL, nil)

	if tagErr != nil {
		return nil, fmt.Errorf("kw getTags request failed: %w", tagErr)
	}

	// 解析标签 — API 返回的 id/digest 是 string 类型，分组用 mdigest 标识
	var tagResp struct {
		Code int `json:"code"`
		Data []struct {
			Name    string `json:"name"`
			MDigest string `json:"mdigest"`
			Img     string `json:"img"`
			Data    []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Digest string `json:"digest"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(tagBody, &tagResp); err != nil {
		return nil, fmt.Errorf("kw getTags parse failed: %w", err)
	}
	if tagResp.Code != 200 {
		return nil, fmt.Errorf("kw getTags API error: code=%d", tagResp.Code)
	}

	var tagGroups []TagGroup
	for i, group := range tagResp.Data {
		if len(group.Data) == 0 {
			continue // 跳过没有子标签的分组（如"有声系列"）
		}
		var items []TagItem
		groupID := fmt.Sprintf("%d", i)
		for _, item := range group.Data {
			items = append(items, TagItem{
				ID:     item.ID + "-" + item.Digest,
				Name:   item.Name,
				Parent: groupID,
			})
		}
		groupName := group.Name
		if groupName == "" {
			groupName = fmt.Sprintf("分组%d", i+1)
		}
		tagGroups = append(tagGroups, TagGroup{
			ID:   groupID,
			Name: groupName,
			List: items,
		})
	}

	// 解析热门标签 — 同样 id/digest 是 string 类型
	var hotTags []TagItem
	if hotErr == nil {
		var hotResp struct {
			Code int `json:"code"`
			Data []struct {
				Data []struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					Digest string `json:"digest"`
				} `json:"data"`
			} `json:"data"`
		}
		if err := json.Unmarshal(hotBody, &hotResp); err == nil && hotResp.Code == 200 && len(hotResp.Data) > 0 {
			for _, item := range hotResp.Data[0].Data {
				hotTags = append(hotTags, TagItem{
					ID:   item.ID + "-" + item.Digest,
					Name: item.Name,
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
func (p *KwSongListProvider) GetList(sortId, tagId string, page int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}
	if sortId == "" {
		sortId = "new"
	}

	var apiURL string
	if tagId == "" {
		// 推荐列表
		apiURL = fmt.Sprintf("http://wapi.kuwo.cn/api/pc/classify/playlist/getRcmPlayList?loginUid=0&loginSid=0&appUid=76039576&&pn=%d&rn=%d&order=%s",
			page, kwSongListLimit, sortId)
	} else {
		// 按标签获取
		parts := strings.SplitN(tagId, "-", 2)
		id := parts[0]
		digest := ""
		if len(parts) > 1 {
			digest = parts[1]
		}

		switch digest {
		case "10000":
			apiURL = fmt.Sprintf("http://wapi.kuwo.cn/api/pc/classify/playlist/getTagPlayList?loginUid=0&loginSid=0&appUid=76039576&pn=%d&id=%s&rn=%d",
				page, id, kwSongListLimit)
		case "43":
			apiURL = fmt.Sprintf("http://mobileinterfaces.kuwo.cn/er.s?type=get_pc_qz_data&f=web&id=%s&prod=pc", id)
		default:
			apiURL = fmt.Sprintf("http://wapi.kuwo.cn/api/pc/classify/playlist/getTagPlayList?loginUid=0&loginSid=0&appUid=76039576&pn=%d&id=%s&rn=%d",
				page, id, kwSongListLimit)
		}
	}

	body, err := HTTPGet(apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kw getList request failed: %w", err)
	}

	// 尝试解析标准格式（注意：API 返回的数值字段实际是 string 类型）
	var stdResp struct {
		Code int `json:"code"`
		Data struct {
			Total int `json:"total"`
			Pn    int `json:"pn"`
			Rn    int `json:"rn"`
			Data  []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Img       string `json:"img"`
				Uname     string `json:"uname"`
				ListenCnt string `json:"listencnt"`
				Total     string `json:"total"`
				FavorCnt  string `json:"favorcnt"`
				Desc      string `json:"desc"`
				Digest    string `json:"digest"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &stdResp); err == nil && stdResp.Code == 200 {
		var list []SongListItem
		for _, item := range stdResp.Data.Data {
			listenCnt, _ := strconv.Atoi(item.ListenCnt)
			favorCnt, _ := strconv.Atoi(item.FavorCnt)
			list = append(list, SongListItem{
				PlayCount: FormatPlayCount(listenCnt),
				ID:        fmt.Sprintf("digest-%s__%s", item.Digest, item.ID),
				Author:    item.Uname,
				Name:      item.Name,
				Total:     item.Total,
				Img:       item.Img,
				Grade:     fmt.Sprintf("%.1f", float64(favorCnt)/10),
				Desc:      item.Desc,
			})
		}
		return &SongListResult{
			List:  list,
			Total: stdResp.Data.Total,
			Page:  stdResp.Data.Pn,
			Limit: stdResp.Data.Rn,
		}, nil
	}

	// 尝试解析 type=43 的数组格式
	var arrResp []struct {
		Label string `json:"label"`
		List  []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Img       string `json:"img"`
			Uname     string `json:"uname"`
			ListenCnt string `json:"listencnt"`
			Total     string `json:"total"`
			FavorCnt  string `json:"favorcnt"`
			Desc      string `json:"desc"`
			Digest    string `json:"digest"`
		} `json:"list"`
	}
	if err := json.Unmarshal(body, &arrResp); err == nil && len(arrResp) > 0 {
		var list []SongListItem
		for _, group := range arrResp {
			if group.Label == "" {
				continue
			}
			for _, item := range group.List {
				listenCnt, _ := strconv.Atoi(item.ListenCnt)
				favorCnt, _ := strconv.Atoi(item.FavorCnt)
				list = append(list, SongListItem{
					PlayCount: FormatPlayCount(listenCnt),
					ID:        fmt.Sprintf("digest-%s__%s", item.Digest, item.ID),
					Author:    item.Uname,
					Name:      item.Name,
					Total:     item.Total,
					Img:       item.Img,
					Grade:     fmt.Sprintf("%.1f", float64(favorCnt)/10),
					Desc:      item.Desc,
				})
			}
		}
		return &SongListResult{
			List:  list,
			Total: 1000,
			Page:  page,
			Limit: 1000,
		}, nil
	}

	return nil, fmt.Errorf("kw getList parse failed")
}

// kwListDetailMusicItem kw 歌单详情中的歌曲项
type kwListDetailMusicItem struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Artist           string `json:"artist"`
	Album            string `json:"album"`
	AlbumID          string `json:"albumid"`
	Duration         string `json:"duration"`
	NMinfo           string `json:"N_MINFO"`
	Pic              string `json:"pic"`
	AlbumPic         string `json:"albumpic"`
	ProbAlbumPic     string `json:"prob_albumpic"`
	WebAlbumPicShort string `json:"web_albumpic_short"`
}

// filterListDetail 转换歌单详情中的歌曲数据
func (p *KwSongListProvider) filterListDetail(items []kwListDetailMusicItem) []SearchItem {
	// 复用 kw_search.go 的 parseMInfo 逻辑
	searcher := &KwSearcher{}

	var list []SearchItem
	for _, item := range items {
		duration, _ := strconv.Atoi(item.Duration)
		singer := DecodeName(strings.ReplaceAll(item.Artist, "&", "、"))

		// 获取封面图
		img := item.Pic
		if img == "" {
			img = item.AlbumPic
		}
		if img == "" {
			img = item.ProbAlbumPic
		}
		if img == "" && item.WebAlbumPicShort != "" {
			img = "https://img4.kuwo.cn/star/albumcover/500" + item.WebAlbumPicShort
		}

		types := searcher.parseMInfo(item.NMinfo)

		list = append(list, SearchItem{
			Singer:  singer,
			Name:    DecodeName(item.Name),
			Album:   DecodeName(item.Album),
			AlbumID: item.AlbumID,
			MusicID: item.ID,
			Songmid: item.ID,
			Source:  "kw",
			Duration: duration,
			Img:     img,
			Types:   types,
		})
	}
	return list
}

// getListDetailDigest8 标准歌单详情获取
func (p *KwSongListProvider) getListDetailDigest8(id string, page int) (*SongListDetailResult, error) {
	if page < 1 {
		page = 1
	}

	apiURL := fmt.Sprintf("http://nplserver.kuwo.cn/pl.svc?op=getlistinfo&pid=%s&pn=%d&rn=%d&encode=utf8&keyset=pl2012&identity=kuwo&pcmp4=1&vipver=MUSIC_9.0.5.0_W1&newver=1",
		id, page-1, kwSongListDetailLimit)

	body, err := HTTPGet(apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kw getListDetailDigest8 request failed: %w", err)
	}

	var resp struct {
		Result    string                  `json:"result"`
		Total     int                     `json:"total"`
		Rn        int                     `json:"rn"`
		Title     string                  `json:"title"`
		Pic       string                  `json:"pic"`
		Info      string                  `json:"info"`
		Uname     string                  `json:"uname"`
		PlayNum   int                     `json:"playnum"`
		MusicList []kwListDetailMusicItem `json:"musiclist"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kw getListDetailDigest8 parse failed: %w", err)
	}
	if resp.Result != "ok" {
		return nil, fmt.Errorf("kw getListDetailDigest8 API error: result=%s", resp.Result)
	}

	list := p.filterListDetail(resp.MusicList)

	return &SongListDetailResult{
		List:  list,
		Total: resp.Total,
		Page:  page,
		Limit: resp.Rn,
		Info: &SongListInfo{
			Name:      resp.Title,
			Img:       resp.Pic,
			Desc:      resp.Info,
			Author:    resp.Uname,
			PlayCount: FormatPlayCount(resp.PlayNum),
		},
	}, nil
}

// getListDetailDigest5Info 获取 digest5 格式的歌单 sourceid
func (p *KwSongListProvider) getListDetailDigest5Info(id string) (string, error) {
	apiURL := fmt.Sprintf("http://qukudata.kuwo.cn/q.k?op=query&cont=ninfo&node=%s&pn=0&rn=1&fmt=json&src=mbox&level=2", id)

	body, err := HTTPGet(apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("kw getListDetailDigest5Info request failed: %w", err)
	}

	var resp struct {
		Child []struct {
			SourceID string `json:"sourceid"`
		} `json:"child"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("kw getListDetailDigest5Info parse failed: %w", err)
	}
	if len(resp.Child) == 0 {
		return "", fmt.Errorf("kw getListDetailDigest5Info: no child found")
	}

	return resp.Child[0].SourceID, nil
}

// getListDetailDigest5 通过 sourceid 获取歌单详情
func (p *KwSongListProvider) getListDetailDigest5(id string, page int) (*SongListDetailResult, error) {
	detailID, err := p.getListDetailDigest5Info(id)
	if err != nil {
		return nil, err
	}
	if detailID == "" {
		return nil, fmt.Errorf("kw getListDetailDigest5: empty sourceid")
	}

	apiURL := fmt.Sprintf("http://nplserver.kuwo.cn/pl.svc?op=getlistinfo&pid=%s&pn=%d&rn=%d&encode=utf-8&keyset=pl2012&identity=kuwo&pcmp4=1",
		detailID, page-1, kwSongListDetailLimit)

	body, err := HTTPGet(apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kw getListDetailDigest5 request failed: %w", err)
	}

	var resp struct {
		Result    string                  `json:"result"`
		Total     int                     `json:"total"`
		Rn        int                     `json:"rn"`
		Title     string                  `json:"title"`
		Pic       string                  `json:"pic"`
		Info      string                  `json:"info"`
		Uname     string                  `json:"uname"`
		PlayNum   int                     `json:"playnum"`
		MusicList []kwListDetailMusicItem `json:"musiclist"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kw getListDetailDigest5 parse failed: %w", err)
	}
	if resp.Result != "ok" {
		return nil, fmt.Errorf("kw getListDetailDigest5 API error: result=%s", resp.Result)
	}

	list := p.filterListDetail(resp.MusicList)

	return &SongListDetailResult{
		List:  list,
		Total: resp.Total,
		Page:  page,
		Limit: resp.Rn,
		Info: &SongListInfo{
			Name:      resp.Title,
			Img:       resp.Pic,
			Desc:      resp.Info,
			Author:    resp.Uname,
			PlayCount: FormatPlayCount(resp.PlayNum),
		},
	}, nil
}

// kwBDListDetailItem bodian 格式歌曲项
type kwBDListDetailItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Album       string `json:"album"`
	AlbumID     int    `json:"albumId"`
	AlbumPic    string `json:"albumPic"`
	Duration    int    `json:"duration"`
	ReleaseDate string `json:"releaseDate"`
	Artists     []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Audios []struct {
		Bitrate string `json:"bitrate"`
		Size    string `json:"size"`
	} `json:"audios"`
}

// filterBDListDetail 转换 bodian 格式歌曲数据
func (p *KwSongListProvider) filterBDListDetail(items []kwBDListDetailItem) []SearchItem {
	var list []SearchItem
	for _, item := range items {
		// 处理歌手
		var singerNames []string
		for _, artist := range item.Artists {
			if artist.Name != "" {
				singerNames = append(singerNames, artist.Name)
			}
		}
		singer := strings.Join(singerNames, "、")

		// 处理音质
		var types []QualityInfo
		for _, audio := range item.Audios {
			size := strings.ToUpper(audio.Size)
			switch audio.Bitrate {
			case "4000":
				types = append(types, QualityInfo{Type: "flac24bit", Size: size})
			case "2000":
				types = append(types, QualityInfo{Type: "flac", Size: size})
			case "320":
				types = append(types, QualityInfo{Type: "320k", Size: size})
			case "128":
				types = append(types, QualityInfo{Type: "128k", Size: size})
			}
		}
		// 反转顺序（低品质在前）
		for i, j := 0, len(types)-1; i < j; i, j = i+1, j-1 {
			types[i], types[j] = types[j], types[i]
		}

		list = append(list, SearchItem{
			Singer:  singer,
			Name:    item.Name,
			Album:   item.Album,
			AlbumID: fmt.Sprintf("%d", item.AlbumID),
			MusicID: fmt.Sprintf("%d", item.ID),
			Songmid: fmt.Sprintf("%d", item.ID),
			Source:  "kw",
			Duration: item.Duration,
			Img:     item.AlbumPic,
			Types:   types,
		})
	}
	return list
}

// getListDetailMusicListByBD 通过 bodian 链接获取歌单详情
func (p *KwSongListProvider) getListDetailMusicListByBD(id string, page int) (*SongListDetailResult, error) {
	reUID := regexp.MustCompile(`uid=(\d+)`)
	reListID := regexp.MustCompile(`playlistId=(\d+)`)
	reSource := regexp.MustCompile(`source=(\d+)`)

	listIDMatch := reListID.FindStringSubmatch(id)
	if listIDMatch == nil {
		return nil, fmt.Errorf("kw getListDetailMusicListByBD: cannot parse playlistId")
	}
	listID := listIDMatch[1]

	source := ""
	sourceMatch := reSource.FindStringSubmatch(id)
	if sourceMatch != nil {
		source = sourceMatch[1]
	}

	uid := ""
	uidMatch := reUID.FindStringSubmatch(id)
	if uidMatch != nil {
		uid = uidMatch[1]
	}

	// 获取歌曲列表
	listURL := fmt.Sprintf("https://bd-api.kuwo.cn/api/service/playlist/%s/musicList?reqId=0&source=%s&pn=%d&rn=%d",
		listID, source, page, kwSongListDetailLimit)

	listBody, err := HTTPGet(listURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36",
		"plat":       "h5",
	})
	if err != nil {
		return nil, fmt.Errorf("kw getListDetailMusicListByBD list request failed: %w", err)
	}

	var listResp struct {
		Code int `json:"code"`
		Data struct {
			Total    int                  `json:"total"`
			PageSize int                  `json:"pageSize"`
			List     []kwBDListDetailItem `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listBody, &listResp); err != nil {
		return nil, fmt.Errorf("kw getListDetailMusicListByBD list parse failed: %w", err)
	}
	if listResp.Code != 200 {
		return nil, fmt.Errorf("kw getListDetailMusicListByBD list API error: code=%d", listResp.Code)
	}

	songs := p.filterBDListDetail(listResp.Data.List)

	// 获取歌单信息
	var info *SongListInfo
	switch source {
	case "4":
		infoURL := fmt.Sprintf("https://bd-api.kuwo.cn/api/service/playlist/info/%s?reqId=0&source=%s", listID, source)
		infoBody, err := HTTPGet(infoURL, map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36",
			"plat":       "h5",
		})
		if err == nil {
			var infoResp struct {
				Code int `json:"code"`
				Data struct {
					Name        string `json:"name"`
					Pic         string `json:"pic"`
					Description string `json:"description"`
					CreatorName string `json:"creatorName"`
					PlayNum     int    `json:"playNum"`
				} `json:"data"`
			}
			if json.Unmarshal(infoBody, &infoResp) == nil && infoResp.Code == 200 {
				info = &SongListInfo{
					Name:      infoResp.Data.Name,
					Img:       infoResp.Data.Pic,
					Desc:      infoResp.Data.Description,
					Author:    infoResp.Data.CreatorName,
					PlayCount: FormatPlayCount(infoResp.Data.PlayNum),
				}
			}
		}
	case "5":
		pubID := uid
		if pubID == "" {
			pubID = listID
		}
		pubURL := fmt.Sprintf("https://bd-api.kuwo.cn/api/ucenter/users/pub/%s?reqId=0", pubID)
		pubBody, err := HTTPGet(pubURL, map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36",
			"plat":       "h5",
		})
		if err == nil {
			var pubResp struct {
				Code int `json:"code"`
				Data struct {
					UserInfo struct {
						Nickname string `json:"nickname"`
						HeadImg  string `json:"headImg"`
					} `json:"userInfo"`
				} `json:"data"`
			}
			if json.Unmarshal(pubBody, &pubResp) == nil && pubResp.Code == 200 {
				info = &SongListInfo{
					Name:   pubResp.Data.UserInfo.Nickname + "喜欢的音乐",
					Img:    pubResp.Data.UserInfo.HeadImg,
					Author: pubResp.Data.UserInfo.Nickname,
				}
			}
		}
	}

	if info == nil {
		info = &SongListInfo{}
	}

	return &SongListDetailResult{
		List:  songs,
		Total: listResp.Data.Total,
		Page:  page,
		Limit: listResp.Data.PageSize,
		Info:  info,
	}, nil
}

// GetListDetail 获取歌单详情（顶层入口方法）
func (p *KwSongListProvider) GetListDetail(id string, page int) (*SongListDetailResult, error) {
	if page < 1 {
		page = 1
	}

	slog.Info("kw getListDetail", "id", id, "page", page)

	// bodian 链接
	if strings.Contains(id, "/bodian/") {
		return p.getListDetailMusicListByBD(id, page)
	}

	// URL 格式
	listDetailLinkRe := regexp.MustCompile(`/playlist(?:_detail)?/(\d+)`)
	if strings.Contains(id, "?") || strings.Contains(id, "&") || strings.Contains(id, ":") || strings.Contains(id, "/") {
		matches := listDetailLinkRe.FindStringSubmatch(id)
		if matches != nil && len(matches) > 1 {
			id = matches[1]
		}
	}

	// digest- 前缀格式
	if strings.HasPrefix(id, "digest-") {
		parts := strings.SplitN(id, "__", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("kw getListDetail: invalid digest id format: %s", id)
		}
		digest := strings.TrimPrefix(parts[0], "digest-")
		realID := parts[1]

		switch digest {
		case "8":
			return p.getListDetailDigest8(realID, page)
		case "5":
			return p.getListDetailDigest5(realID, page)
		default:
			return p.getListDetailDigest5(realID, page)
		}
	}

	// 默认使用 digest8
	return p.getListDetailDigest8(id, page)
}

// SearchSongList 搜索歌单
func (p *KwSongListProvider) SearchSongList(keyword string, page int, limit int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	apiURL := fmt.Sprintf("http://search.kuwo.cn/r.s?all=%s&pn=%d&rn=%d&rformat=json&encoding=utf8&ver=mbox&vipver=MUSIC_8.7.7.0_BCS37&plat=pc&devid=28156413&ft=playlist&pay=0&needliveshow=0",
		url.QueryEscape(keyword), page-1, limit)

	slog.Info("kw searchSongList", "keyword", keyword, "page", page, "url", apiURL)

	body, err := HTTPGet(apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("kw searchSongList request failed: %w", err)
	}

	// kw 搜索返回的可能是非标准 JSON，需要预处理
	searcher := &KwSearcher{}
	jsonStr := searcher.fixJsonFormat(string(body))

	var resp struct {
		Total   string `json:"TOTAL"`
		Abslist []struct {
			PlayCnt    string `json:"playcnt"`
			PlaylistID string `json:"playlistid"`
			Nickname   string `json:"nickname"`
			Name       string `json:"name"`
			SongNum    string `json:"songnum"`
			Pic        string `json:"pic"`
			Intro      string `json:"intro"`
		} `json:"abslist"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("kw searchSongList parse failed: %w", err)
	}

	total, _ := strconv.Atoi(resp.Total)

	var list []SongListItem
	for _, item := range resp.Abslist {
		playCnt, _ := strconv.Atoi(item.PlayCnt)
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(playCnt),
			ID:        item.PlaylistID,
			Author:    DecodeName(item.Nickname),
			Name:      DecodeName(item.Name),
			Total:     item.SongNum,
			Img:       item.Pic,
			Desc:      DecodeName(item.Intro),
		})
	}

	return &SongListResult{
		List:  list,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}
