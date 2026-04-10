//go:build wasip1

package musicsdk

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// KgSongListProvider kg 平台歌单提供者
type KgSongListProvider struct{}

// NewKgSongListProvider 创建 kg 平台歌单提供者
func NewKgSongListProvider() *KgSongListProvider {
	return &KgSongListProvider{}
}

// ID 返回平台标识
func (p *KgSongListProvider) ID() string {
	return "kg"
}

// Name 返回平台名称
func (p *KgSongListProvider) Name() string {
	return "kg"
}

// kg 歌单相关常量
const (
	kgSongListSignKey     = "NVPh5oo715z5DIWAeQlhMDsWXXQV4hwt"
	kgSongListLimit       = 30
	kgSongListDetailLimit = 100
)

// GetSortList 返回排序选项
func (p *KgSongListProvider) GetSortList() []SortItem {
	return []SortItem{
		{ID: "5", Name: "推荐"},
		{ID: "6", Name: "最热"},
		{ID: "7", Name: "最新"},
		{ID: "3", Name: "热藏"},
	}
}

// kgSignatureParams 生成 kg 签名参数，参考 kugou.go 的 signKugouSonginfoParams
func kgSignatureParams(params map[string]string) string {
	pairs := make([]string, 0, len(params))
	for key, value := range params {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	data := kgSongListSignKey + strings.Join(pairs, "") + kgSongListSignKey
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GetTags 获取歌单标签
func (p *KgSongListProvider) GetTags() (*TagResult, error) {
	apiURL := "http://www2.kugou.kugou.com/yueku/v9/special/getSpecial?is_smarty=1&cdn=cdn&t=5&c=0"

	body, err := HTTPGet(apiURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36",
	})
	if err != nil {
		return nil, fmt.Errorf("kg getTags request failed: %w", err)
	}

	var resp struct {
		Status int `json:"status"`
		Data   struct {
			HotTag json.RawMessage `json:"hotTag"`
			TagIDs json.RawMessage `json:"tagids"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kg getTags parse failed: %w", err)
	}
	if resp.Status != 1 {
		return nil, fmt.Errorf("kg getTags API error: status=%d", resp.Status)
	}

	// 转换热门标签 — hotTag 是 object（{"0": {...}, "1": {...}}）
	var hotTags []TagItem
	if resp.Data.HotTag != nil {
		var hotTagMap map[string]struct {
			SpecialName string `json:"special_name"`
			SpecialID   string `json:"special_id"`
			ID          int    `json:"id"`
		}
		if json.Unmarshal(resp.Data.HotTag, &hotTagMap) == nil {
			for _, item := range hotTagMap {
				hotTags = append(hotTags, TagItem{
					ID:   item.SpecialID,
					Name: item.SpecialName,
				})
			}
		}
	}

	// 转换分组标签 — tagids 是 object（{"主题": {"id": 1, "data": [...]}, ...}）
	var tagGroups []TagGroup
	if resp.Data.TagIDs != nil {
		var tagIDMap map[string]struct {
			ID   int `json:"id"`
			Data []struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				ParentID int    `json:"parent_id"`
			} `json:"data"`
		}
		if json.Unmarshal(resp.Data.TagIDs, &tagIDMap) == nil {
			for groupName, group := range tagIDMap {
				var items []TagItem
				for _, item := range group.Data {
					items = append(items, TagItem{
						ID:     fmt.Sprintf("%d", item.ID),
						Name:   item.Name,
						Parent: fmt.Sprintf("%d", group.ID),
					})
				}
				tagGroups = append(tagGroups, TagGroup{
					ID:   fmt.Sprintf("%d", group.ID),
					Name: groupName,
					List: items,
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
func (p *KgSongListProvider) GetList(sortId, tagId string, page int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}

	// 如果没有标签且是第一页且是默认排序，同时获取推荐列表
	var recommendList []SongListItem
	if tagId == "" && page == 1 && (sortId == "" || sortId == "5") {
		recList, _ := p.getSongListRecommend()
		recommendList = recList
	}

	list, total, err := p.getSongList(sortId, tagId, page)
	if err != nil {
		return nil, err
	}

	// 合并推荐列表
	if len(recommendList) > 0 {
		list = append(recommendList, list...)
	}

	return &SongListResult{
		List:  list,
		Total: total,
		Page:  page,
		Limit: kgSongListLimit,
	}, nil
}

// getSongList 获取歌单列表数据
// 使用 m.kugou.com 移动端 API（PC 端 getSpecial API 已不再返回歌单列表）
func (p *KgSongListProvider) getSongList(sortId, tagId string, page int) ([]SongListItem, int, error) {
	// m.kugou.com 的 plist 接口支持 category 参数过滤标签
	apiURL := fmt.Sprintf("http://m.kugou.com/plist/index&json=true&page=%d&pagesize=%d",
		page, kgSongListLimit)
	if tagId != "" {
		apiURL += "&category=" + tagId
	}

	body, err := HTTPGet(apiURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1",
		"Referer":    "http://m.kugou.com",
	})
	if err != nil {
		return nil, 0, fmt.Errorf("kg getSongList request failed: %w", err)
	}

	var resp struct {
		Plist struct {
			List struct {
				Total   int `json:"total"`
				HasNext int `json:"has_next"`
				Info    []struct {
					SpecialID   int    `json:"specialid"`
					SpecialName string `json:"specialname"`
					ImgURL      string `json:"imgurl"`
					PlayCount   int    `json:"playcount"`
					SongCount   int    `json:"songcount"`
					Username    string `json:"username"`
					Intro       string `json:"intro"`
					PublishTime string `json:"publishtime"`
				} `json:"info"`
			} `json:"list"`
			PageSize int `json:"pagesize"`
		} `json:"plist"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("kg getSongList parse failed: %w", err)
	}

	var list []SongListItem
	for _, item := range resp.Plist.List.Info {
		img := item.ImgURL
		if img != "" {
			img = strings.ReplaceAll(img, "{size}", "240")
		}
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(item.PlayCount),
			ID:        fmt.Sprintf("id_%d", item.SpecialID),
			Author:    item.Username,
			Name:      item.SpecialName,
			Time:      item.PublishTime,
			Img:       img,
			Desc:      item.Intro,
			Total:     fmt.Sprintf("%d", item.SongCount),
		})
	}

	return list, resp.Plist.List.Total, nil
}

// getSongListRecommend 获取推荐歌单
func (p *KgSongListProvider) getSongListRecommend() ([]SongListItem, error) {
	apiURL := "http://everydayrec.service.kugou.com/guess_special_recommend"

	reqBody := `{"appid":1001,"clienttime":0,"clientver":0,"key":"","mid":"","platform":"pc","userid":0}`
	body, err := HTTPPostJSON(apiURL, []byte(reqBody), map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36",
	})
	if err != nil {
		return nil, fmt.Errorf("kg getSongListRecommend request failed: %w", err)
	}

	var resp struct {
		Status int `json:"status"`
		Data   struct {
			SpecialList []struct {
				SpecialID   int    `json:"specialid"`
				SpecialName string `json:"specialname"`
				ImgURL      string `json:"imgurl"`
				PlayCount   int    `json:"playcount"`
				TotalCount  int    `json:"totalcount"`
				Nickname    string `json:"nickname"`
				Intro       string `json:"intro"`
			} `json:"special_list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kg getSongListRecommend parse failed: %w", err)
	}

	var list []SongListItem
	for _, item := range resp.Data.SpecialList {
		img := item.ImgURL
		if img != "" {
			img = strings.ReplaceAll(img, "{size}", "240")
		}
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(item.PlayCount),
			ID:        fmt.Sprintf("id_%d", item.SpecialID),
			Author:    item.Nickname,
			Name:      item.SpecialName,
			Img:       img,
			Desc:      item.Intro,
			Total:     fmt.Sprintf("%d", item.TotalCount),
		})
	}

	return list, nil
}

// kgSongListDetailItem 歌单详情中的歌曲项（mobilecdn API）
type kgSongListDetailItem struct {
	Hash         string `json:"hash"`
	Songname     string `json:"songname"`
	Singername   string `json:"singername"`
	Filename     string `json:"filename"`
	AlbumName    string `json:"album_name"`
	AlbumID      string `json:"album_id"`
	AudioID      int    `json:"audio_id"`
	Duration     int    `json:"duration"`
	Img          string `json:"img"`
	AlbumImg     string `json:"album_img"`
	Filesize     int64  `json:"filesize"`
	Filesize320  int64  `json:"filesize_320"`
	Hash320      string `json:"hash_320"`
	FilesizeApe  int64  `json:"filesize_ape"`
	HashApe      string `json:"hash_ape"`
	FilesizeFlac int64  `json:"filesize_flac"`
	HashFlac     string `json:"hash_flac"`
}

// kgSongListDetailItem2 歌单详情中的歌曲项（pubsongscdn API）
type kgSongListDetailItem2 struct {
	AudioInfo struct {
		AudioID      string `json:"audio_id"`
		Hash         string `json:"hash"`
		Hash320      string `json:"hash_320"`
		HashFlac     string `json:"hash_flac"`
		HashHigh     string `json:"hash_high"`
		Filesize     string `json:"filesize"`
		Filesize320  string `json:"filesize_320"`
		FilesizeFlac string `json:"filesize_flac"`
		FilesizeHigh string `json:"filesize_high"`
		Timelength   string `json:"timelength"`
		TransParam   struct {
			UnionCover string `json:"union_cover"`
		} `json:"trans_param"`
	} `json:"audio_info"`
	AuthorName string `json:"author_name"`
	Songname   string `json:"songname"`
	AlbumInfo  struct {
		AlbumName    string `json:"album_name"`
		AlbumID      string `json:"album_id"`
		SizableCover string `json:"sizable_cover"`
		Pic          string `json:"pic"`
		Img          string `json:"img"`
		SImg         string `json:"s_img"`
	} `json:"album_info"`
	Img string `json:"img"`
}

// filterData 转换歌曲数据（mobilecdn 格式）为 SearchItem
func (p *KgSongListProvider) filterData(items []kgSongListDetailItem) []SearchItem {
	slog.Info("kg filterData", "itemCount", len(items))
	var list []SearchItem
	for i, item := range items {
		var types []QualityInfo
		if item.Filesize != 0 {
			types = append(types, QualityInfo{
				Type: "128k",
				Size: SizeToStr(item.Filesize),
				Hash: item.Hash,
			})
		}
		if item.Filesize320 != 0 {
			types = append(types, QualityInfo{
				Type: "320k",
				Size: SizeToStr(item.Filesize320),
				Hash: item.Hash320,
			})
		}
		if item.FilesizeApe != 0 {
			types = append(types, QualityInfo{
				Type: "ape",
				Size: SizeToStr(item.FilesizeApe),
				Hash: item.HashApe,
			})
		}
		if item.FilesizeFlac != 0 {
			types = append(types, QualityInfo{
				Type: "flac",
				Size: SizeToStr(item.FilesizeFlac),
				Hash: item.HashFlac,
			})
		}

		img := item.Img
		if img == "" {
			img = item.AlbumImg
		}
		if img != "" {
			img = strings.ReplaceAll(img, "{size}", "400")
		}

		// 解析歌手和歌曲名：优先使用独立的 songname/singername，否则从 filename 解析
		singer := item.Singername
		songName := item.Songname
		slog.Debug("kg filterData parse", "index", i, "hash", item.Hash, "songname", item.Songname, "singername", item.Singername, "filename", item.Filename)
		if singer == "" && songName == "" && item.Filename != "" {
			parts := strings.SplitN(item.Filename, " - ", 2)
			if len(parts) == 2 {
				singer = parts[0]
				slog.Debug("kg filterData parsed from filename", "index", i, "singer", singer, "songName", songName)
			}
			songName = item.Filename
		}

		list = append(list, SearchItem{
			Singer:   DecodeName(singer),
			Name:     DecodeName(songName),
			Album:    DecodeName(item.AlbumName),
			AlbumID:  item.AlbumID,
			MusicID:  item.Hash,
			Source:   "kg",
			Duration: item.Duration / 1000,
			Img:      img,
			Hash:     item.Hash,
			Types:    types,
		})
	}
	slog.Info("kg filterData done", "resultCount", len(list))
	return list
}

// filterData2 转换歌曲数据（pubsongscdn 格式）为 SearchItem
func (p *KgSongListProvider) filterData2(items []kgSongListDetailItem2) []SearchItem {
	ids := make(map[string]bool)
	var list []SearchItem
	for _, item := range items {
		audioID := item.AudioInfo.AudioID
		if audioID == "" || ids[audioID] {
			continue
		}
		ids[audioID] = true

		var types []QualityInfo
		if item.AudioInfo.Filesize != "0" && item.AudioInfo.Filesize != "" {
			size, _ := strconv.ParseInt(item.AudioInfo.Filesize, 10, 64)
			types = append(types, QualityInfo{
				Type: "128k",
				Size: SizeToStr(size),
				Hash: item.AudioInfo.Hash,
			})
		}
		if item.AudioInfo.Filesize320 != "0" && item.AudioInfo.Filesize320 != "" {
			size, _ := strconv.ParseInt(item.AudioInfo.Filesize320, 10, 64)
			types = append(types, QualityInfo{
				Type: "320k",
				Size: SizeToStr(size),
				Hash: item.AudioInfo.Hash320,
			})
		}
		if item.AudioInfo.FilesizeFlac != "0" && item.AudioInfo.FilesizeFlac != "" {
			size, _ := strconv.ParseInt(item.AudioInfo.FilesizeFlac, 10, 64)
			types = append(types, QualityInfo{
				Type: "flac",
				Size: SizeToStr(size),
				Hash: item.AudioInfo.HashFlac,
			})
		}
		if item.AudioInfo.FilesizeHigh != "0" && item.AudioInfo.FilesizeHigh != "" {
			size, _ := strconv.ParseInt(item.AudioInfo.FilesizeHigh, 10, 64)
			types = append(types, QualityInfo{
				Type: "flac24bit",
				Size: SizeToStr(size),
				Hash: item.AudioInfo.HashHigh,
			})
		}

		img := item.Img
		if img == "" {
			img = item.AlbumInfo.SizableCover
		}
		if img == "" {
			img = item.AudioInfo.TransParam.UnionCover
		}
		if img == "" {
			img = item.AlbumInfo.Pic
		}
		if img == "" {
			img = item.AlbumInfo.Img
		}
		if img == "" {
			img = item.AlbumInfo.SImg
		}
		if img != "" {
			img = strings.ReplaceAll(img, "{size}", "400")
		}

		duration := 0
		if tl, err := strconv.Atoi(item.AudioInfo.Timelength); err == nil {
			duration = tl / 1000
		}

		list = append(list, SearchItem{
			Singer:   DecodeName(item.AuthorName),
			Name:     DecodeName(item.Songname),
			Album:    DecodeName(item.AlbumInfo.AlbumName),
			AlbumID:  item.AlbumInfo.AlbumID,
			MusicID:  item.AudioInfo.Hash,
			Source:   "kg",
			Duration: duration,
			Img:      img,
			Hash:     item.AudioInfo.Hash,
			Types:    types,
		})
	}
	return list
}

// getListDetailBySpecialId 通过 specialId 获取歌单详情，参考 kugou.go 的 fetchPlaylistDetail
func (p *KgSongListProvider) getListDetailBySpecialId(id string, page int) (*SongListDetailResult, error) {
	if page < 1 {
		page = 1
	}

	apiURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/special/song?specialid=%s&area_code=1&page=%d&plat=2&pagesize=%d&version=8000",
		id, page, kgSongListDetailLimit)

	body, err := HTTPGet(apiURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1",
	})
	if err != nil {
		return nil, fmt.Errorf("kg getListDetailBySpecialId request failed: %w", err)
	}

	var resp struct {
		Status int `json:"status"`
		Data   struct {
			Total int                    `json:"total"`
			Info  []kgSongListDetailItem `json:"info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kg getListDetailBySpecialId parse failed: %w", err)
	}
	if resp.Status != 1 {
		return nil, fmt.Errorf("kg getListDetailBySpecialId API error: status=%d", resp.Status)
	}

	list := p.filterData(resp.Data.Info)

	return &SongListDetailResult{
		List:  list,
		Total: resp.Data.Total,
		Page:  page,
		Limit: kgSongListDetailLimit,
	}, nil
}

// getUserListDetailById 通过 specialId + signature 获取歌单详情
func (p *KgSongListProvider) getUserListDetailById(id string, page int) ([]SearchItem, error) {
	if page < 1 {
		page = 1
	}

	params := map[string]string{
		"srcappid":  "2919",
		"clientver": "20000",
		"appid":     "1058",
		"type":      "0",
		"module":    "playlist",
		"page":      fmt.Sprintf("%d", page),
		"pagesize":  fmt.Sprintf("%d", kgSongListDetailLimit),
		"specialid": id,
	}
	signature := kgSignatureParams(params)

	apiURL := fmt.Sprintf("https://pubsongscdn.kugou.com/v2/get_other_list_file?srcappid=2919&clientver=20000&appid=1058&type=0&module=playlist&page=%d&pagesize=%d&specialid=%s&signature=%s",
		page, kgSongListDetailLimit, id, signature)

	body, err := HTTPGet(apiURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/11.0 Mobile/15A372 Safari/604.1",
		"Referer":    "https://m3ws.kugou.com/share/index.php",
		"dfid":       "-",
	})
	if err != nil {
		return nil, fmt.Errorf("kg getUserListDetailById request failed: %w", err)
	}

	var resp struct {
		Status  int                     `json:"status"`
		ErrCode int                     `json:"errcode"`
		Info    []kgSongListDetailItem2 `json:"info"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kg getUserListDetailById parse failed: %w", err)
	}

	return p.filterData2(resp.Info), nil
}

// getUserListDetail2 通过 global_collection_id 获取歌单详情
func (p *KgSongListProvider) getUserListDetail2(globalCollectionID string) (*SongListDetailResult, error) {
	apiURL := fmt.Sprintf("https://pubsongscdn.kugou.com/v2/get_other_list_file?srcappid=2919&clientver=20000&appid=1058&type=0&module=playlist&global_collection_id=%s&page=1&pagesize=99999",
		globalCollectionID)

	body, err := HTTPGet(apiURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38",
		"Referer":    "https://m3ws.kugou.com/share/index.php",
		"dfid":       "-",
	})
	if err != nil {
		return nil, fmt.Errorf("kg getUserListDetail2 request failed: %w", err)
	}

	var resp struct {
		Status  int                     `json:"status"`
		ErrCode int                     `json:"errcode"`
		Info    []kgSongListDetailItem2 `json:"info"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kg getUserListDetail2 parse failed: %w", err)
	}

	list := p.filterData2(resp.Info)

	return &SongListDetailResult{
		List:  list,
		Total: len(list),
		Page:  1,
		Limit: len(list),
	}, nil
}

// getUserListDetail3 通过 chain 获取歌单详情（分页）
func (p *KgSongListProvider) getUserListDetail3(chain string, page int) (*SongListDetailResult, error) {
	// 先获取歌单信息
	info, err := p.getListInfoByChain(chain)
	if err != nil {
		slog.Warn("kg getUserListDetail3 getListInfoByChain failed", "chain", chain, "error", err)
	}

	// 如果有 specialid，使用 getUserListDetailById
	if info != nil && info.specialID != "" {
		songs, err := p.getUserListDetailById(info.specialID, page)
		if err != nil {
			return nil, err
		}

		var songListInfo *SongListInfo
		if info != nil {
			img := info.imgURL
			if img != "" {
				img = strings.ReplaceAll(img, "{size}", "240")
			}
			songListInfo = &SongListInfo{
				Name:   info.specialName,
				Img:    img,
				Author: info.nickname,
			}
		}

		return &SongListDetailResult{
			List:  songs,
			Info:  songListInfo,
			Total: len(songs),
			Page:  page,
			Limit: kgSongListDetailLimit,
		}, nil
	}

	return nil, fmt.Errorf("kg getUserListDetail3: cannot get specialid from chain %s", chain)
}

// kgChainInfo 歌单链信息
type kgChainInfo struct {
	specialID   string
	specialName string
	imgURL      string
	nickname    string
}

// getListInfoByChain 通过 chain 获取歌单信息，参考 kugou.go 的 fetchSonglistDetail
func (p *KgSongListProvider) getListInfoByChain(chain string) (*kgChainInfo, error) {
	pageURL := fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", chain)

	body, err := HTTPGet(pageURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36",
	})
	if err != nil {
		return nil, fmt.Errorf("kg getListInfoByChain request failed: %w", err)
	}

	bodyStr := string(body)

	// 解析 window.$output 中的 JSON 数据
	re := regexp.MustCompile(`var\s+specialData\s*=\s*(\{.+?\});`)
	matches := re.FindStringSubmatch(bodyStr)
	if matches == nil {
		// 尝试另一种模式
		re2 := regexp.MustCompile(`window\.\$output\s*=\s*(\{.+?\});`)
		matches = re2.FindStringSubmatch(bodyStr)
	}

	if matches != nil && len(matches) > 1 {
		var data struct {
			SpecialID   int    `json:"specialid"`
			SpecialName string `json:"specialname"`
			ImgURL      string `json:"imgurl"`
			Nickname    string `json:"nickname"`
		}
		if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
			return &kgChainInfo{
				specialID:   fmt.Sprintf("%d", data.SpecialID),
				specialName: data.SpecialName,
				imgURL:      data.ImgURL,
				nickname:    data.Nickname,
			}, nil
		}
	}

	return nil, fmt.Errorf("kg getListInfoByChain: cannot parse page data")
}

// decodeGcid 解码 gcid
func (p *KgSongListProvider) decodeGcid(gcid string) (string, error) {
	apiURL := fmt.Sprintf("https://gateway.kugou.com/openapi/kmr/v1/collect/gcid/decode?gcid=%s", gcid)

	body, err := HTTPGet(apiURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38",
	})
	if err != nil {
		return "", fmt.Errorf("kg decodeGcid request failed: %w", err)
	}

	var resp struct {
		ErrCode int `json:"errcode"`
		Data    struct {
			GlobalCollectionID string `json:"global_collection_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("kg decodeGcid parse failed: %w", err)
	}
	if resp.ErrCode != 0 || resp.Data.GlobalCollectionID == "" {
		return "", fmt.Errorf("kg decodeGcid failed: errcode=%d", resp.ErrCode)
	}

	return resp.Data.GlobalCollectionID, nil
}

// getUserListDetailByCode 通过纯数字 ID 获取歌单详情
func (p *KgSongListProvider) getUserListDetailByCode(id string) (*SongListDetailResult, error) {
	return p.getListDetailBySpecialId(id, 1)
}

// GetListDetail 获取歌单详情（顶层入口方法）
func (p *KgSongListProvider) GetListDetail(id string, page int) (*SongListDetailResult, error) {
	if page < 1 {
		page = 1
	}

	slog.Info("kg getListDetail", "id", id, "page", page)

	// 处理 special/single/ 格式
	if strings.Contains(id, "special/single/") {
		re := regexp.MustCompile(`special/single/(\w+)`)
		matches := re.FindStringSubmatch(id)
		if matches != nil && len(matches) > 1 {
			id = matches[1]
		}
	}

	// 处理 URL 格式
	if strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://") {
		return p.getUserListDetail(id, page)
	}

	// 纯数字 ID
	if matched, _ := regexp.MatchString(`^\d+$`, id); matched {
		return p.getUserListDetailByCode(id)
	}

	// id_ 前缀
	if strings.HasPrefix(id, "id_") {
		id = strings.TrimPrefix(id, "id_")
		return p.getListDetailBySpecialId(id, page)
	}

	// 默认当作 specialId
	return p.getListDetailBySpecialId(id, page)
}

// getUserListDetail 处理 URL 格式的歌单详情
func (p *KgSongListProvider) getUserListDetail(link string, page int) (*SongListDetailResult, error) {
	// 去除 hash 部分
	if idx := strings.Index(link, "#"); idx != -1 {
		link = link[:idx]
	}

	// 处理 global_collection_id 参数
	if strings.Contains(link, "global_collection_id") {
		re := regexp.MustCompile(`global_collection_id=(\w+)`)
		matches := re.FindStringSubmatch(link)
		if matches != nil && len(matches) > 1 {
			return p.getUserListDetail2(matches[1])
		}
	}

	// 处理 gcid_ 参数
	if strings.Contains(link, "gcid_") {
		re := regexp.MustCompile(`gcid_\w+`)
		gcid := re.FindString(link)
		if gcid != "" {
			globalCollectionID, err := p.decodeGcid(gcid)
			if err == nil && globalCollectionID != "" {
				return p.getUserListDetail2(globalCollectionID)
			}
		}
	}

	// 处理 chain= 参数
	if strings.Contains(link, "chain=") {
		re := regexp.MustCompile(`chain=(\w+)`)
		matches := re.FindStringSubmatch(link)
		if matches != nil && len(matches) > 1 {
			return p.getUserListDetail3(matches[1], page)
		}
	}

	// 处理 .html 链接
	if strings.Contains(link, ".html") {
		if strings.Contains(link, "zlist.html") {
			// zlist 格式
			newLink := strings.Replace(link, "zlist.html", "", 1)
			newLink = fmt.Sprintf("https://m3ws.kugou.com/zlist/list%s&pagesize=%d&page=%d",
				newLink[strings.Index(newLink, "?"):], kgSongListDetailLimit, page)
			return p.getUserListDetail(newLink, page)
		}
		if !strings.Contains(link, "song.html") {
			// 普通 .html 链接，提取 chain
			re := regexp.MustCompile(`.+/(\w+)\.html`)
			matches := re.FindStringSubmatch(link)
			if matches != nil && len(matches) > 1 {
				return p.getUserListDetail3(matches[1], page)
			}
		}
	}

	// 尝试请求链接获取重定向
	body, err := HTTPGet(link, map[string]string{
		"User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1",
		"Referer":    link,
	})
	if err != nil {
		return nil, fmt.Errorf("kg getUserListDetail request failed: %w", err)
	}

	// 尝试从响应体中提取 global_collection_id
	bodyStr := string(body)
	re := regexp.MustCompile(`"global_collection_id":"(\w+)"`)
	matches := re.FindStringSubmatch(bodyStr)
	if matches != nil && len(matches) > 1 {
		return p.getUserListDetail2(matches[1])
	}

	// 尝试提取 encode_gic 或 encode_src_gid
	reGcid := regexp.MustCompile(`"encode_gic":"(\w+)"`)
	gcidMatches := reGcid.FindStringSubmatch(bodyStr)
	if gcidMatches == nil {
		reGcid2 := regexp.MustCompile(`"encode_src_gid":"(\w+)"`)
		gcidMatches = reGcid2.FindStringSubmatch(bodyStr)
	}
	if gcidMatches != nil && len(gcidMatches) > 1 {
		globalCollectionID, err := p.decodeGcid(gcidMatches[1])
		if err == nil && globalCollectionID != "" {
			return p.getUserListDetail2(globalCollectionID)
		}
	}

	return nil, fmt.Errorf("kg getUserListDetail: cannot parse link %s", link)
}

// SearchSongList 搜索歌单
func (p *KgSongListProvider) SearchSongList(keyword string, page int, limit int) (*SongListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}

	apiURL := fmt.Sprintf("http://msearchretry.kugou.com/api/v3/search/special?keyword=%s&page=%d&pagesize=%d&showtype=10&filter=0&version=7910&sver=2",
		url.QueryEscape(keyword), page, limit)

	slog.Info("kg searchSongList", "keyword", keyword, "page", page, "url", apiURL)

	body, err := HTTPGet(apiURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36",
	})
	if err != nil {
		return nil, fmt.Errorf("kg searchSongList request failed: %w", err)
	}

	var resp struct {
		ErrCode int `json:"errcode"`
		Data    struct {
			Total int `json:"total"`
			Info  []struct {
				SpecialID   int    `json:"specialid"`
				SpecialName string `json:"specialname"`
				ImgURL      string `json:"imgurl"`
				PlayCount   int    `json:"playcount"`
				SongCount   int    `json:"songcount"`
				Nickname    string `json:"nickname"`
				Intro       string `json:"intro"`
				Grade       int    `json:"grade"`
				PublishTime string `json:"publishtime"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kg searchSongList parse failed: %w", err)
	}
	if resp.ErrCode != 0 {
		return nil, fmt.Errorf("kg searchSongList API error: errcode=%d", resp.ErrCode)
	}

	var list []SongListItem
	for _, item := range resp.Data.Info {
		list = append(list, SongListItem{
			PlayCount: FormatPlayCount(item.PlayCount),
			ID:        fmt.Sprintf("id_%d", item.SpecialID),
			Author:    item.Nickname,
			Name:      item.SpecialName,
			Time:      item.PublishTime,
			Img:       item.ImgURL,
			Grade:     fmt.Sprintf("%d", item.Grade),
			Desc:      item.Intro,
			Total:     fmt.Sprintf("%d", item.SongCount),
		})
	}

	return &SongListResult{
		List:  list,
		Total: resp.Data.Total,
		Page:  page,
		Limit: limit,
	}, nil
}
