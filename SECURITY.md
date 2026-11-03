# 安全

## 报告安全问题

请不要在 issue 里公开。到仓库的 [Security → Report a vulnerability](https://github.com/renyijiu/tingyu/security/advisories/new) 私下报告，只有维护者能看到。

报告里请写清楚：

- 问题是什么，会造成什么后果；
- 怎么重现，最好附上能触发它的 RSS 地址或文件；
- 听宇版本和 macOS 版本。

确认问题后，会在这份私下的报告里商量怎么修，修好后发新版本，再公开说明。你愿意的话，会在 release 说明里致谢。

只修最新发布的版本。

## 听宇接触到的数据

- **Podcast Index 的 API Key 和 Secret**：在设置里保存时写进 `~/Library/Application Support/Tingyu/config.json`，只有你自己能读写（0600）；也可以只放在环境变量里，见 README 的「配置 Podcast Index」。密钥只用来给发往 `api.podcastindex.org` 的请求签名。
- **订阅、播放进度和最近播放**：在同一目录的 `library.json`，只在本机。
- **缓存**：节目的 RSS、封面和音频在 `~/Library/Caches/Tingyu`，只在本机。

听宇会下载并解析任意节目的 RSS、封面和音频，这些都来自不受信任的网站。构造出来的 RSS、图片或音频能让听宇读写这些目录以外的文件、执行代码，或把上面的数据发到别处，都算安全问题。只让听宇崩溃或卡住的，直接提 issue 就行。
