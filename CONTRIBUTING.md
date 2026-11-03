# 参与听宇

欢迎提 issue 和 PR。README 里「还没做」那一列都可以认领，动手前在 issue 里说一声，免得撞车。

## 开始之前

- 需要 Go 1.27.3 或更高版本和一台 Mac。`go tool mygo dev` 运行开发版，改了 Go 文件会自动重启。
- 开发时最好用独立的数据目录，不碰自己真正的订阅：`TINGYU_DATA_DIR="$HOME/Library/Application Support/Tingyu-Dev" go tool mygo dev`。
- 改界面前读一遍[界面设计说明](docs/design.md)：颜色、字号、动效和性能都有约定，界面测试也依赖其中列出的文字。

## 提交 PR 前

```bash
gofmt -l .
go vet ./...
go test -race ./...
```

- 新功能和修复都带上测试。界面背后的逻辑放在 `internal/app` 等包里，可以不开窗口测；界面本身用 mygo 的 `ui.Tester` 测。
- 改了 `go.mod` 的依赖，运行 `go run ./tools/notices` 更新第三方许可声明。
- 只能在真实窗口里看出来的改动（动画、星云、声音），在 PR 里写清楚怎么验证的。
- 注释和提交说明写这段代码为什么这样做，用平实的话。

## 密钥

不要提交 Podcast Index 的密钥。`.env` 和 `config.json` 已在 `.gitignore` 里。

发现安全问题请按 [SECURITY.md](SECURITY.md) 私下报告。
