//go:build wasip1

package musicsdk

// SearchResult 搜索结果
type SearchResult struct {
	List  []SearchItem `json:"list"`
	Total int          `json:"total"`
	Page  int          `json:"page"`
	Limit int          `json:"limit"`
}

// SearchItem 搜索项（需兼容不同平台的字段）
type SearchItem struct {
	Name     string        `json:"name"`
	Singer   string        `json:"singer"`
	Album    string        `json:"album"`
	AlbumID  string        `json:"albumId,omitempty"`
	Duration int           `json:"duration"`        // 秒
	Source   string        `json:"source"`          // 平台标识 kg/kw/tx/wy/mg
	MusicID  string        `json:"musicId"`         // 平台歌曲唯一标识
	Img      string        `json:"img,omitempty"`   // 封面图 URL
	Types    []QualityInfo `json:"types,omitempty"` // 可用音质列表

	// 平台特有字段（getMusicUrl 时需要）
	Hash        string `json:"hash,omitempty"`        // kg
	CopyrightId string `json:"copyrightId,omitempty"` // mg
	StrMediaMid string `json:"strMediaMid,omitempty"` // tx
	AlbumMid    string `json:"albumMid,omitempty"`    // tx
	Songmid     string `json:"songmid,omitempty"`     // tx/wy
}

// QualityInfo 音质信息
type QualityInfo struct {
	Type string `json:"type"`           // "128k", "320k", "flac", "flac24bit"
	Size string `json:"size,omitempty"` // 文件大小（可选）
	Hash string `json:"hash,omitempty"` // 文件 hash（kg 特有）
}
