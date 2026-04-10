//go:build wasip1

package musicsdk

// Searcher 搜索器接口
type Searcher interface {
	ID() string
	Name() string
	Search(keyword string, page int, limit int) (*SearchResult, error)
}

// PlatformInfo 平台信息（用于前端展示）
type PlatformInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SongListProvider 歌单提供者接口
type SongListProvider interface {
	ID() string
	Name() string
	GetSortList() []SortItem
	GetList(sortId, tagId string, page int) (*SongListResult, error)
	GetTags() (*TagResult, error)
	GetListDetail(id string, page int) (*SongListDetailResult, error)
	SearchSongList(keyword string, page int, limit int) (*SongListResult, error)
}

// Registry 搜索器注册表
type Registry struct {
	searchers         map[string]Searcher
	lyricFetchers     map[string]LyricFetcher
	songListProviders map[string]SongListProvider
	order             []string // 保持注册顺序
}

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		searchers:         make(map[string]Searcher),
		lyricFetchers:     make(map[string]LyricFetcher),
		songListProviders: make(map[string]SongListProvider),
		order:             []string{},
	}
}

// Register 注册搜索器
func (r *Registry) Register(s Searcher) {
	id := s.ID()
	if _, exists := r.searchers[id]; !exists {
		r.order = append(r.order, id)
	}
	r.searchers[id] = s
}

// Get 获取指定 ID 的搜索器
func (r *Registry) Get(id string) (Searcher, bool) {
	s, ok := r.searchers[id]
	return s, ok
}

// All 返回有序的平台列表
func (r *Registry) All() []PlatformInfo {
	platforms := make([]PlatformInfo, 0, len(r.order))
	for _, id := range r.order {
		if s, ok := r.searchers[id]; ok {
			platforms = append(platforms, PlatformInfo{
				ID:   s.ID(),
				Name: s.Name(),
			})
		}
	}
	return platforms
}

// RegisterLyricFetcher 注册歌词获取器
func (r *Registry) RegisterLyricFetcher(f LyricFetcher) {
	r.lyricFetchers[f.ID()] = f
}

// GetLyricFetcher 获取指定 ID 的歌词获取器
func (r *Registry) GetLyricFetcher(id string) (LyricFetcher, bool) {
	f, ok := r.lyricFetchers[id]
	return f, ok
}

// RegisterSongListProvider 注册歌单提供者
func (r *Registry) RegisterSongListProvider(p SongListProvider) {
	r.songListProviders[p.ID()] = p
}

// GetSongListProvider 获取指定 ID 的歌单提供者
func (r *Registry) GetSongListProvider(id string) (SongListProvider, bool) {
	p, ok := r.songListProviders[id]
	return p, ok
}

// AllSongListProviders 返回所有歌单提供者的平台信息
func (r *Registry) AllSongListProviders() []PlatformInfo {
	platforms := make([]PlatformInfo, 0, len(r.songListProviders))
	for _, id := range r.order {
		if p, ok := r.songListProviders[id]; ok {
			platforms = append(platforms, PlatformInfo{
				ID:   p.ID(),
				Name: p.Name(),
			})
		}
	}
	return platforms
}
