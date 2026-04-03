//go:build wasip1

package musicsdk

import (
	"encoding/json"
	"fmt"
	"log/slog"
)

const (
	txAPIURL   = "https://u.y.qq.com/cgi-bin/musicu.fcg"
	txUA       = "QQMusic 14090508(android 12)"
	txSourceID = "tx"
)

// TxSearcher tx 平台搜索器
type TxSearcher struct{}

// NewTxSearcher 创建 tx 平台搜索器
func NewTxSearcher() *TxSearcher {
	return &TxSearcher{}
}

// ID 返回平台 ID
func (s *TxSearcher) ID() string {
	return txSourceID
}

// Name 返回平台名称
func (s *TxSearcher) Name() string {
	return "tx"
}

// txSearchRequest 搜索请求结构
type txSearchRequest struct {
	Comm txComm    `json:"comm"`
	Req  txReqBody `json:"req"`
}

type txComm struct {
	CT             string `json:"ct"`
	CV             string `json:"cv"`
	V              string `json:"v"`
	TmeAppID       string `json:"tmeAppID"`
	Phonetype      string `json:"phonetype"`
	DeviceScore    string `json:"deviceScore"`
	Devicelevel    string `json:"devicelevel"`
	Newdevicelevel string `json:"newdevicelevel"`
	Rom            string `json:"rom"`
	OsVer          string `json:"os_ver"`
	OpenUDID       string `json:"OpenUDID"`
	OpenUDID2      string `json:"OpenUDID2"`
	QIMEI36        string `json:"QIMEI36"`
	Udid           string `json:"udid"`
	Chid           string `json:"chid"`
	Aid            string `json:"aid"`
	Oaid           string `json:"oaid"`
	Taid           string `json:"taid"`
	Tid            string `json:"tid"`
	Wid            string `json:"wid"`
	UID            string `json:"uid"`
	Sid            string `json:"sid"`
	ModeSwitch     string `json:"modeSwitch"`
	TeenMode       string `json:"teenMode"`
	UIMode         string `json:"ui_mode"`
	Nettype        string `json:"nettype"`
	V4ip           string `json:"v4ip"`
}

type txReqBody struct {
	Method string  `json:"method"`
	Module string  `json:"module"`
	Param  txParam `json:"param"`
}

type txParam struct {
	SearchType int    `json:"search_type"`
	Query      string `json:"query"`
	PageNum    int    `json:"page_num"`
	NumPerPage int    `json:"num_per_page"`
	Highlight  int    `json:"highlight"`
	NqcFlag    int    `json:"nqc_flag"`
	MultiZhida int    `json:"multi_zhida"`
	Cat        int    `json:"cat"`
	Grp        int    `json:"grp"`
	Sin        int    `json:"sin"`
	Sem        int    `json:"sem"`
}

// txSearchResponse 搜索响应结构
type txSearchResponse struct {
	Code int `json:"code"`
	Req  struct {
		Code int `json:"code"`
		Data struct {
			Meta struct {
				EstimateSum int `json:"estimate_sum"`
			} `json:"meta"`
			Body struct {
				ItemSong []txSongItem `json:"item_song"`
			} `json:"body"`
		} `json:"data"`
	} `json:"req"`
}

type txSongItem struct {
	Name     string `json:"name"`
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

// Search 执行搜索
func (s *TxSearcher) Search(keyword string, page int, limit int) (*SearchResult, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}

	// 构造请求体（参数与 lxserver 保持一致）
	reqBody := txSearchRequest{
		Comm: txComm{
			CT:             "11",
			CV:             "14090508",
			V:              "14090508",
			TmeAppID:       "qqmusic",
			Phonetype:      "EBG-AN10",
			DeviceScore:    "553.47",
			Devicelevel:    "50",
			Newdevicelevel: "20",
			Rom:            "HuaWei/EMOTION/EmotionUI_14.2.0",
			OsVer:          "12",
			OpenUDID:       "0",
			OpenUDID2:      "0",
			QIMEI36:        "0",
			Udid:           "0",
			Chid:           "0",
			Aid:            "0",
			Oaid:           "0",
			Taid:           "0",
			Tid:            "0",
			Wid:            "0",
			UID:            "0",
			Sid:            "0",
			ModeSwitch:     "6",
			TeenMode:       "0",
			UIMode:         "2",
			Nettype:        "1020",
			V4ip:           "",
		},
		Req: txReqBody{
			Module: "music.search.SearchCgiService",
			Method: "DoSearchForQQMusicMobile",
			Param: txParam{
				SearchType: 0,
				Query:      keyword,
				PageNum:    page,
				NumPerPage: limit,
				Highlight:  0,
				NqcFlag:    0,
				MultiZhida: 0,
				Cat:        2,
				Grp:        1,
				Sin:        0,
				Sem:        0,
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// 发送请求
	headers := map[string]string{
		"User-Agent": txUA,
	}
	respBytes, err := HTTPPostJSON(txAPIURL, bodyBytes, headers)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	// 调试日志：打印原始响应
	slog.Info("tx search", "keyword", keyword, "page", page, "respLen", len(respBytes))
	if len(respBytes) <= 2000 {
		slog.Debug("tx search raw response", "body", string(respBytes))
	} else {
		slog.Debug("tx search raw response (truncated)", "body", string(respBytes[:2000]))
	}

	// 解析响应
	var resp txSearchResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 调试日志：打印解析结果
	slog.Info("tx search result",
		"code", resp.Code,
		"req.code", resp.Req.Code,
		"total", resp.Req.Data.Meta.EstimateSum,
		"items", len(resp.Req.Data.Body.ItemSong))

	// 检查响应状态
	if resp.Code != 0 || resp.Req.Code != 0 {
		return nil, fmt.Errorf("api error: code=%d, req.code=%d", resp.Code, resp.Req.Code)
	}

	// 转换结果
	items := make([]SearchItem, 0, len(resp.Req.Data.Body.ItemSong))
	for _, song := range resp.Req.Data.Body.ItemSong {
		// 跳过没有 media_mid 的歌曲
		if song.File.MediaMid == "" {
			continue
		}

		item := s.convertSongItem(&song)
		items = append(items, item)
	}

	return &SearchResult{
		List:  items,
		Total: resp.Req.Data.Meta.EstimateSum,
		Page:  page,
		Limit: limit,
	}, nil
}

// convertSongItem 转换歌曲项
func (s *TxSearcher) convertSongItem(song *txSongItem) SearchItem {
	// 拼接歌手名
	singerNames := make([]string, 0, len(song.Singer))
	for _, singer := range song.Singer {
		singerNames = append(singerNames, singer.Name)
	}

	// 生成封面 URL
	var img string
	albumMid := song.Album.Mid
	if albumMid != "" && albumMid != "空" {
		img = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R500x500M000%s.jpg", albumMid)
	} else if len(song.Singer) > 0 && song.Singer[0].Mid != "" {
		img = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T001R500x500M000%s.jpg", song.Singer[0].Mid)
	}

	// 构建音质列表
	types := make([]QualityInfo, 0, 4)
	file := song.File
	if file.Size128mp3 > 0 {
		types = append(types, QualityInfo{
			Type: "128k",
			Size: SizeToStr(file.Size128mp3),
		})
	}
	if file.Size320mp3 > 0 {
		types = append(types, QualityInfo{
			Type: "320k",
			Size: SizeToStr(file.Size320mp3),
		})
	}
	if file.SizeFlac > 0 {
		types = append(types, QualityInfo{
			Type: "flac",
			Size: SizeToStr(file.SizeFlac),
		})
	}
	if file.SizeHires > 0 {
		types = append(types, QualityInfo{
			Type: "flac24bit",
			Size: SizeToStr(file.SizeHires),
		})
	}

	return SearchItem{
		Name:        DecodeName(song.Name),
		Singer:      FormatSingers(singerNames),
		Album:       DecodeName(song.Album.Name),
		AlbumID:     song.Album.Mid,
		Duration:    song.Interval,
		Source:      txSourceID,
		MusicID:     song.Mid,
		Img:         img,
		Types:       types,
		Songmid:     song.Mid,
		AlbumMid:    albumMid,
		StrMediaMid: song.File.MediaMid,
	}
}
