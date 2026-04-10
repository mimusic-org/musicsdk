//go:build wasip1

package musicsdk

// SongListItem 歌单列表项
type SongListItem struct {
	PlayCount   string `json:"play_count"`            // 播放量（已格式化）
	ID          string `json:"id"`                     // 歌单 ID
	Author      string `json:"author"`                 // 作者
	Name        string `json:"name"`                   // 歌单名称
	Time        string `json:"time,omitempty"`         // 创建/更新时间
	Img         string `json:"img,omitempty"`          // 封面图 URL
	Grade       string `json:"grade,omitempty"`        // 评分
	Desc        string `json:"desc,omitempty"`         // 描述
	Total       string `json:"total,omitempty"`        // 歌曲总数
	PlayCountRaw int   `json:"play_count_raw,omitempty"` // 原始播放量数值
}

// SongListResult 歌单列表结果
type SongListResult struct {
	List  []SongListItem `json:"list"`
	Total int            `json:"total"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
}

// SongListInfo 歌单详情信息
type SongListInfo struct {
	Name        string `json:"name"`                   // 歌单名称
	Img         string `json:"img,omitempty"`           // 封面图 URL
	Desc        string `json:"desc,omitempty"`          // 描述
	Author      string `json:"author,omitempty"`        // 作者
	PlayCount   string `json:"play_count,omitempty"`    // 播放量
	Total       int    `json:"total,omitempty"`         // 歌曲总数
}

// SongListDetailResult 歌单详情结果（含歌曲列表）
type SongListDetailResult struct {
	List  []SearchItem  `json:"list"`            // 歌曲列表（复用 SearchItem）
	Info  *SongListInfo `json:"info,omitempty"`  // 歌单信息
	Total int           `json:"total"`           // 歌曲总数
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

// TagItem 标签项
type TagItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Parent string `json:"parent,omitempty"` // 父分组 ID
}

// TagGroup 标签分组
type TagGroup struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	List []TagItem `json:"list"`
}

// TagResult 标签结果
type TagResult struct {
	Tags []TagGroup `json:"tags"` // 按分组组织的标签
	Hot  []TagItem  `json:"hot,omitempty"` // 热门标签（扁平列表）
}

// SortItem 排序选项
type SortItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
