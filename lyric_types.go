//go:build wasip1

package musicsdk

// LyricResult 歌词获取结果
type LyricResult struct {
	Lyric   string `json:"lyric"`   // 标准 LRC 歌词
	TLyric  string `json:"tlyric"`  // 翻译歌词
	RLyric  string `json:"rlyric"`  // 罗马音歌词
	LxLyric string `json:"lxlyric"` // 逐字歌词
}

// LyricFetcher 歌词获取器接口
type LyricFetcher interface {
	// ID 返回平台标识
	ID() string
	// GetLyric 获取歌词
	// songInfo 包含平台所需的歌曲信息字段
	GetLyric(songInfo map[string]interface{}) (*LyricResult, error)
}
