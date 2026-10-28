# 听宇 Tingyu

An open-source macOS podcast client written in Go.

听宇是一个用纯 Go 写的 macOS 桌面播客客户端。界面使用 [mygo](https://github.com/egoist/mygo) 的原生 UI（不嵌网页），节目数据来自 [Podcast Index](https://podcastindex.org/)，没有密钥时搜索会改走 iTunes。

## 这个版本能做什么

- 用中文或英文关键词搜索播客
- 订阅和取消订阅，列表保存在本机
- 查看一个节目的单集（优先解析它自己的 RSS）
- 播放单集音频：播放、暂停、拖动进度，并记住每集听到哪里

还没做：小宇宙同步、下载管理、账号。播放时会把音频缓存在数据目录里，方便拖动进度，这不是给用户看的下载列表。当前解码支持 MP3 和 WAV。MP3 解码器首次打开时会扫描完整文件，因此可能需要等下载完成才开始播放；已完成的缓存可以复用。切换单集或退出时会取消旧下载，未完成的文件不会作为完整缓存保留。

## 配置 Podcast Index

到 [Podcast Index](https://api.podcastindex.org/) 申请免费的 API key 和 secret。没有密钥也可以打开应用，搜索会使用 iTunes。

任选一种方式，环境变量优先于 `.env`，`.env` 优先于配置文件：

```bash
export PODCASTINDEX_API_KEY="你的 key"
export PODCASTINDEX_API_SECRET="你的 secret"
```

或者在项目目录放一个不会被提交的 `.env`（可从 `.env.example` 复制）。

或者编辑数据目录里的 `config.json`。macOS 上这个目录是：

```text
~/Library/Application Support/Tingyu/config.json
```

```json
{
  "podcastIndexKey": "你的 key",
  "podcastIndexSecret": "你的 secret"
}
```

第一次启动时如果文件还不存在，应用会写一份空模板。密钥不要提交到仓库。

订阅和播放进度在同一目录的 `library.json`。播放缓存放在其中的 `cache/`。用 `TINGYU_DATA_DIR` 可以改掉这个目录（测试时会用到）。

## 在 Mac 上构建和运行

需要 Go 1.27.3 或更高版本。mygo 不使用 cgo。

```bash
go test ./...
go tool mygo dev
```

`go tool mygo dev` 会编译开发版并在改 Go 文件后重启。开发版是一个独立的 app bundle，但默认与正式版共用数据目录。开发时可显式隔离数据：

```bash
TINGYU_DATA_DIR="$HOME/Library/Application Support/Tingyu-Dev" go tool mygo dev
```

打包：

```bash
go tool mygo build
```

在 Mac 上，产物在 `build/darwin-<arch>/`，包括 `听宇.app` 和 dmg。发给别人用之前需要签名和公证，见 [mygo 的打包文档](https://mygo.egoist.dev/docs/distribution)。dmg 和签名只能在 macOS 上完成。

其他系统也可以编出 macOS 的 `.app`（mygo 不需要 cgo）：

```bash
go tool mygo build -platform darwin/arm64 -skip-dmg
```

只编译当前平台的可执行文件、不打安装包时：

```bash
go build -o tingyu .
```

## 代码是怎么分开的

| 包 | 职责 |
| --- | --- |
| `internal/podcastindex` | Podcast Index 客户端，含鉴权签名 |
| `internal/itunes` | iTunes Search API，无密钥 |
| `internal/feed` | 用 gofeed 解析 RSS |
| `internal/catalog` | 搜索和单集的来源选择 |
| `internal/store` | 订阅和播放进度的 JSON 文件 |
| `internal/media` | 远端音频下载、可 seek 的文件读取和完整缓存复用 |
| `internal/player` | 播放、暂停、进度记忆；音频引擎是接口 |
| `internal/player/mygoengine` | 用 mygo `audio` 真正出声 |
| `internal/app` | 界面背后的状态 |
| `internal/ui` | mygo 原生界面 |
| `main.go` | 把上面接起来，打开窗口 |

## 许可

本仓库的代码使用 MIT 许可证。直接依赖的 mygo 和 gofeed 也是 MIT，可以一起用。播放 MP3 时会经过 mygo 带到 `github.com/hajimehoshi/go-mp3`，那个库是 Apache-2.0，只约束它自己的代码。
