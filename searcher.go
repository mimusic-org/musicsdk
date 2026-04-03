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

// Registry 搜索器注册表
type Registry struct {
	searchers     map[string]Searcher
	lyricFetchers map[string]LyricFetcher
	order         []string // 保持注册顺序
}

// NewRegistry 创建新的注册表
func NewRegistry() *Registry {
	return &Registry{
		searchers:     make(map[string]Searcher),
		lyricFetchers: make(map[string]LyricFetcher),
		order:         []string{},
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
