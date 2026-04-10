# musicsdk

MiMusic 音乐搜索、歌词与歌单 SDK，为 WASM 插件提供多平台音乐搜索、歌词获取和歌单管理能力。

## 支持平台

| 平台 ID | 搜索 | 歌词 | 歌单 |
|---------|:----:|:----:|:----:|
| `kg`    | ✅ | ✅ | ✅ |
| `kw`    | ✅ | ✅ | ✅ |
| `tx`    | ✅ | ✅ | ✅ |
| `wy`    | ✅ | ✅ | ✅ |
| `mg`    | ✅ | ✅ | ✅ |

## 核心接口

### Searcher — 搜索器

```go
type Searcher interface {
    ID() string
    Name() string
    Search(keyword string, page int, limit int) (*SearchResult, error)
}
```

### LyricFetcher — 歌词获取器

```go
type LyricFetcher interface {
    ID() string
    GetLyric(songInfo map[string]interface{}) (*LyricResult, error)
}
```

### SongListProvider — 歌单提供者

```go
type SongListProvider interface {
    ID() string
    Name() string
    GetSortList() []SortItem
    GetList(sortId, tagId string, page int) (*SongListResult, error)
    GetTags() (*TagResult, error)
    GetListDetail(id string, page int) (*SongListDetailResult, error)
    SearchSongList(keyword string, page int, limit int) (*SongListResult, error)
}
```

### Registry — 注册表

统一管理所有平台的搜索器、歌词获取器和歌单提供者，支持按 ID 获取和有序遍历：

```go
registry := musicsdk.NewRegistry()

// 搜索器
registry.Register(musicsdk.NewKgSearcher())

// 歌词获取器
registry.RegisterLyricFetcher(musicsdk.NewKgLyricFetcher())

// 歌单提供者
registry.RegisterSongListProvider(musicsdk.NewKgSongListProvider())
```

## 数据结构

### 搜索相关

- **`SearchResult`** — 搜索结果（包含分页信息和 `SearchItem` 列表）
- **`SearchItem`** — 搜索项（歌名、歌手、专辑、时长、音质列表及各平台特有字段）
- **`QualityInfo`** — 音质信息（128k / 320k / flac / flac24bit）
- **`LyricResult`** — 歌词结果（标准 LRC、翻译、罗马音、逐字歌词）

### 歌单相关

- **`SongListItem`** — 歌单项（ID、名称、封面、播放量、作者、描述）
- **`SongListResult`** — 歌单列表结果（包含分页信息和 `SongListItem` 列表）
- **`SongListDetailResult`** — 歌单详情结果（歌曲列表 `[]SearchItem` + 歌单信息 `SongListInfo`）
- **`SongListInfo`** — 歌单信息（名称、封面、描述、作者、播放量）
- **`TagItem`** — 标签项（ID、名称）
- **`TagGroup`** — 标签分组（分组名 + 标签列表）
- **`TagResult`** — 标签结果（分组标签 + 热门标签）
- **`SortItem`** — 排序选项（ID、名称）

## HTTP 工具

封装了 WASM 环境下的 HTTP 请求方法（基于 `go-plugin-http`，通过 Host Function 代理网络请求）：

- `HTTPGet(url, headers)` — GET 请求
- `HTTPPost(url, body, headers)` — POST 请求
- `HTTPPostJSON(url, body, headers)` — POST JSON
- `HTTPPostForm(url, body, headers)` — POST Form

## 通用工具函数

- `FormatPlayTime(seconds)` — 秒数格式化为 `MM:SS`
- `FormatPlayCount(num)` — 播放量格式化（如 `1.2万`、`3.4亿`）
- `DateFormat(timestamp, format)` — 时间戳格式化
- `FormatSingers(names)` — 歌手名拼接（`A、B、C`）
- `DecodeName(s)` — HTML 实体解码
- `SizeToStr(size)` — 文件大小格式化（KB/MB/GB）

## 加密工具

wy 平台专用加密函数，WASM 环境兼容（使用 `math/big` 替代 `crypto/rsa`）：

- `weapiEncrypt(data)` — WeAPI 加密（AES-CBC 双重加密 + RSA）
- `linuxapiEncrypt(data)` — LinuxAPI 加密（AES-ECB）
- `eapiEncrypt(url, data)` — EAPI 加密（AES-ECB）

## 构建约束

所有源文件均使用 `//go:build wasip1` 构建标签，仅在 WASM (WASI) 目标下编译。

> **注意**：WASM 环境不支持标准库 `net/http`，必须使用 `go-plugin-http` 进行网络请求。

## 免责声明

- 本项目**仅供个人学习研究技术使用**，严禁任何形式的商业用途，包括但不限于售卖、牟利，不得使用本代码进行任何形式的牟利/贩卖/传播。
- 本项目数据来源原理是从各第三方音源中获取数据，因此本项目不对数据的准确性、合法性负责。使用本项目的过程中可能会产生版权数据，对于这些版权数据，本项目不拥有它们的所有权，为了避免侵权，使用者务必在 24 小时内清除使用本项目过程中所产生的版权数据。
- 本项目完全免费，仅供个人私下范围研究交流学习技术使用，并开源发布于 GitHub 面向全世界人用作对技术的学习交流，本项目不对项目内的技术可能存在违反当地法律法规的行为作保证，禁止在违反当地法律法规的情况下使用本项目，对于使用者在明知或不知当地法律法规不允许的情况下使用本项目所造成的任何违法违规行为由使用者承担，本项目不承担由此造成的任何直接、间接、特殊、偶然或结果性责任。
- 若你使用了本项目，将代表你接受以上声明。

> **注意**：本项目仅作为示例项目用于学习研究，请勿用于任何商业或违法用途。如有侵犯到任何人的合法权益，请联系作者，将在第一时间修改删除相关代码。

## 许可证

[Apache License 2.0](LICENSE)
