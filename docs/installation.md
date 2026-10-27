# 开发测试包的构建与安装

**这是 M1 开发包，不是已通过 KPW5/Vera 实机验收的版本。** 当前能在电脑浏览器运行阅读核心；起点、长佩在线解析仍为实验状态；MiuRead 的登录/书架代码已移植，真实访问未验证，正文解码尚未完成。包内没有示例书冒充平台作品。

## 在云环境构建

```sh
sh scripts/setup.sh
sh scripts/check.sh
npm run test:browser
sh scripts/build.sh
```

`setup.sh` 校验并安装 Go 1.25.3 到项目 `.tools/`，不会覆盖系统 `/usr/bin/go`。构建依赖 Node/npm、Python3、curl、tar、sha256sum 和 file。Go 缓存放在 `/tmp`。浏览器测试自动使用 `/usr/bin/chromium`；其他环境可设置 `CHROMIUM_PATH` 或按 Playwright 文档安装 Chromium。

产物：`dist/kindle-read-0.1.0-alpha.1-armv7.zip`、`dist/kindle-read-source.tar.gz`、`dist/SHA256SUMS`。ARM 程序静态编译，不依赖 Kindle 上的 Go、Node、Python、KUAL 或 KOReader。

本地运行：

```sh
sh scripts/go.sh run ./cmd/kindle-read serve --source qidian --data .local-data
# 将 qidian 改为 changpei 或 miuread，分别打开三个应用。
```

程序输出本机私有 URL；只监听 `127.0.0.1`。调试链接具有临时会话路径，不要公开分享。运行数据位于 `.local-data/`，已加入忽略规则。

## Kindle 上的安装路径

首个目标设备是 KPW5/Vera，但固件及 Vera 版本尚未获得。以下过程来自社区 Scriptlet/Mesquite 机制，**尚未在目标设备执行**。程序会检查 Mesquite、sqlite3、lipc-set-prop 与 appreg 表结构；预检通过仍不证明书架索引、WebKit、触摸或休眠兼容。

1. 在电脑验证 ZIP 的 SHA-256。保留设备原有文件；首次测试前保存设备备份。
2. 将 ZIP 中的 `extensions/kindle-read` 和 `documents/KindleRead-Setup.sh`、`documents/KindleRead-Uninstall.sh` 合并复制到 Kindle 用户分区的同名目录。升级前先退出三个应用，保留 `extensions/kindle-read/data`。
3. 安全弹出 USB。Vera Scriptlets 可用时，书架应显示「安装 Kindle Read 测试版」。点一次完成组件检查、注册及三个入口创建。若看不到安装入口，停止在此步骤，核实 Vera/固件的 Scriptlet 支持；不自动改装其他启动工具。
4. 返回书架，分别点击「起点阅读」「长佩阅读」「MiuRead 独立版」。首次显示各自书架；已有本地阅读记录时尝试续读。

安装仅写入本项目的 `/var/local/mesquite/io.github.xinhang725.kindleread.*`、`/mnt/us/documents/KindleRead-*.sh`，以及 `/var/local/appreg.db` 中本项目三个 ID 的注册。**不重挂载、不写系统根分区。** 注册使用 SQLite 事务；普通错误会回滚本轮文件变更，断电跨文件/数据库恢复仍需实机验证。遇到已存在的非项目目录、被修改的入口或注册冲突时拒绝覆盖。

启动链：书架 Scriptlet → 独立 ARM helper → appmgr → Mesquite → 本地界面。没有应用选择菜单。能否达到“一点即用”仍以设备上的测试结果为准。

## 当前可测试的功能

- 在每个应用内导入自有 UTF-8 文本，验证分页、字号、目录、退出和再次打开。它只是核心验证，不算三平台阅读闭环。
- 起点/长佩可提交官方作品或章节链接，程序只尝试读取公开 HTML；动态目录、付费正文、挑战页面可能返回不支持。当前不提供这两个平台的账号登录。
- MiuRead 可发起真实接口的二维码登录，确认后读取账号书架；未迁移章节解码，点击线上作品会明确说明暂不可阅读。不会调用购买接口或自动创建 Skills API key。
- 账号状态不进入界面响应或普通日志。文件权限请求为 `0600/0700`，但 Kindle 用户分区可能为 FAT，**不提供加密或 USB 访问隔离**；持有设备的人仍可读取会话文件。

## 已知限制及恢复

- 当前云环境平台主机被出站策略阻止；环境设置已保存域名草稿，需要保存、发布后才可做真实联网检查。设备本身的联网不由云草稿控制。
- TLS 使用设备系统证书并检查证书与主机名；旧证书库或错误设备时间会导致联网失败，不提供关闭校验选项。
- helper 在连续五分钟无请求后退出。界面不可见时不发保活；休眠唤醒后的服务恢复、实际休眠行为和耗电尚未适配/验证。若唤醒后提示连接中断，退出并从书架重新打开；这项限制意味着 A14 尚未通过。
- 数据默认缓存上限为每账号 100 MiB；当前到限时报错，尚无自动淘汰。单章最多 2 MiB；极长文本的旧设备排版性能未测量。
- 数据损坏会显示错误并保留原文件，不会静默重建空书架。日志位于 `extensions/kindle-read/data/run/<来源>.log`，不要上传 `auth.json` 或运行会话 `.json`。
- 若启动失败，先确认完整固件、Vera 版本和报错。不要通过卸载 KOReader、清空 appreg 数据库或替换系统组件尝试修复。

## 卸载

先从三个应用中退出，再点「卸载 Kindle Read 入口」。安装/卸载会检查所有应用的进程锁；若提示仍在运行，逐个打开后退出，或等待五分钟空闲超时后重试。卸载只移除项目创建的三个入口、Mesquite 配置和三项注册，**保留 `extensions/kindle-read/data`**。被用户编辑过的入口会阻止卸载，先自行保存并处理该入口，避免误删。

卸载成功后，可通过 USB 删除 `extensions/kindle-read`、安装/卸载脚本；这最后一步会同时删除保留的本项目账号、缓存和进度，请按需备份。其余书籍和应用不在卸载范围。
