# 听宇 Tingyu

An open-source macOS podcast client written in Go.

听宇是一个用纯 Go 写的 macOS 桌面播客客户端。界面使用 [mygo](https://github.com/egoist/mygo) 的原生 UI（不嵌网页），节目数据来自 [Podcast Index](https://podcastindex.org/)，没有密钥时搜索会改走 iTunes。

名字取「听见宇宙」之意：界面是一片夜空，节目封面的颜色会染进背景的星云，侧栏和播放栏是悬浮在上面的毛玻璃，深浅色跟随系统。设计约定见[界面设计说明](docs/design.md)。

## 这个版本能做什么

- 用中文或英文关键词搜索播客
- 订阅和取消订阅，列表保存在本机
- 查看一个节目的单集（优先解析它自己的 RSS）
- 播放单集音频：播放、暂停、拖动进度、前后跳 15/30 秒、0.8×–2× 倍速，并记住每集听到哪里
- 在「正在播放」页里，封面化作一颗行星，外圈轨道显示进度，光晕随声音的响度起伏

还没做：小宇宙同步、下载管理、账号。播放时会把音频缓存在数据目录里，方便拖动进度，这不是给用户看的下载列表。当前解码支持 MP3、M4A（MP4 里的 AAC，小宇宙的节目都是这种）和 WAV。M4A 用 macOS 自带的 AudioToolbox 解码：索引（`moov`）在文件开头时边下边播，小宇宙的文件就是这样；索引在末尾时要等下载完成。MP3 解码器首次打开时会扫描完整文件，因此可能需要等下载完成才开始播放；已完成的缓存可以复用。切换单集或退出时会取消旧下载，未完成的文件不会作为完整缓存保留。

## 配置 Podcast Index

到 [Podcast Index](https://api.podcastindex.org/) 申请免费的 API key 和 secret。没有密钥也可以打开应用，搜索会使用 iTunes。

打开侧栏底部的齿轮「设置」，或选择菜单「听宇 → 设置…」（快捷键 `⌘,`）；在没有密钥时，点「发现」页搜索框下的提示条也能打开。填写 **API Key** 和 **API Secret**，点击「保存」即可。两项默认隐藏，保存后立即用于后续搜索，无需重启。将两项都清空并保存可移除本机密钥，恢复使用 iTunes（存在环境覆盖时除外）。保存只确认本机写入成功，不验证密钥是否有效。

密钥以仅当前用户可读写的权限保存在本机 `config.json` 中。也兼容下面的配置方式：环境变量优先于 `.env`，`.env` 优先于应用内设置和配置文件；有环境覆盖时设置窗口会提示，且不会把环境中的密钥复制到本机配置。

```bash
export PODCASTINDEX_API_KEY="你的 key"
export PODCASTINDEX_API_SECRET="你的 secret"
```

或者在项目目录放一个不会被提交的 `.env`（可从 `.env.example` 复制）。

也可以手动编辑数据目录里的 `config.json`。macOS 上这个文件是：

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

需要 Go 1.27.3 或更高版本。mygo 不使用 cgo，听宇也不用：M4A 解码像 mygo 的音频输出一样，通过 purego 调用系统的 AudioToolbox。

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

应用图标 `resources/icon.png` 由代码绘制，和侧栏的标志是同一份。改了 `internal/ui/brand.go` 里的标志后重新生成：

```bash
go run ./tools/icon
```

## 测试与验收

运行自动回归和静态检查：

```bash
go test -race ./...
go vet ./...
```

真实数据、原生窗口和异常播放的验证结果，以及隔离数据目录的手动复测步骤，见[首版验收记录](docs/testing/2026-10-28-v1-acceptance.md)。界面改版后的验证结果和需要人工检查的视觉项，见[界面改版验收记录](docs/testing/2026-10-28-ui-redesign.md)。小宇宙 M4A 播放的验证见 [M4A 播放验收记录](docs/testing/2026-10-28-m4a.md)。

界面测试用的渲染器没有 GPU，星云着色器和持续的动画不会运行；它们只能在真实窗口里检查。

## 代码是怎么分开的

| 包 | 职责 |
| --- | --- |
| `internal/podcastindex` | Podcast Index 客户端，含鉴权签名 |
| `internal/itunes` | iTunes Search API，无密钥 |
| `internal/feed` | 用 gofeed 解析 RSS |
| `internal/catalog` | 搜索和单集的来源选择 |
| `internal/store` | 订阅和播放进度的 JSON 文件 |
| `internal/media` | 远端音频下载、可 seek 的文件读取和完整缓存复用 |
| `internal/player` | 播放、暂停、进度记忆、按播放位置查询响度；音频引擎是接口 |
| `internal/player/mygoengine` | 用 mygo `audio` 真正出声，解码时测量响度 |
| `internal/player/mygoengine/m4a` | 通过 purego 调用 macOS AudioToolbox 解码 M4A（AAC），读到还没下载的部分时等待 |
| `internal/app` | 界面背后的状态 |
| `internal/ui` | mygo 原生界面：星云背景着色器（`nebula.kage`）、图标、动效和各页面 |
| `tools/icon` | 用界面里的标志绘制应用图标 `resources/icon.png`（`go run ./tools/icon`） |
| `main.go` | 把上面接起来，打开窗口 |

## 许可

本仓库的代码使用 MIT 许可证。直接依赖的 mygo 和 gofeed 也是 MIT，可以一起用。播放 MP3 时会经过 mygo 带到 `github.com/hajimehoshi/go-mp3`，那个库是 Apache-2.0，只约束它自己的代码。

M4A 解码直接依赖 `github.com/ebitengine/purego`，它是 Apache-2.0，mygo 本身也依赖它，同样只约束它自己的代码。真正解码的是 macOS 自带的 AudioToolbox，它是系统框架，不随应用分发，不带来新的许可条款。没有用纯 Go 的 AAC 解码库，是因为找到的几个都是 LGPL 或 GPL：Go 是静态链接的，和 MIT 的应用放在一起不方便。
