# 0.1.0-alpha.1 实现与验证状态

2026-10-27，目标 KPW5/Vera。当前是 **M1 开发测试版本**，三个应用的完整真实阅读闭环尚未完成，不能作为日常阅读稳定版发布。

## 已落地代码

| 边界 | 当前实现 | 证据与限制 |
| --- | --- | --- |
| 三个独立入口 | 三个 Scriptlet、独立 appmgr/Mesquite ID；分别启动来源上下文 | 临时系统根/真实 SQLite 测试；没有 Kindle 实机结果 |
| 独立运行时 | 静态 Go ARM helper + 本地 ES5 界面，回环随机端口与随机会话路径 | 桌面真实 HTTP 测试、静态 ARM ELF；无 KOReader 库或下载步骤 |
| 阅读核心 | 目录、按 DOM 测量分页、中文/emoji、字号、段落及 UTF-16 锚点、本地续读 | Node 逻辑及 Chromium 交互测试；旧 WebKit/墨水屏速度未测 |
| 数据 | 来源/账号分区，书架与进度原子写入；缓存限制与损坏报错 | 临时目录测试、写失败/数据隔离测试；实际断电未测试 |
| 起点 | 官方链接校验、公开 HTML 目录/章节解析 | **合成结构夹具通过**；真实 HTML、目录完整性、正文完整性未核对；未实现账号/付费章节 |
| 长佩 | 官方链接校验、公开 HTML 目录/章节解析 | 同上，单独夹具；不能引用起点结果宣布长佩可用 |
| MiuRead | 固定上游 Web QR 流程、指纹、Cookie 边界、验证码、取消/过期、会话提交、两步账号书架接口 | **HTTP 协议夹具通过**；没有真实授权/书架结果；正文解码未迁移 |
| 安装 | 组件/表结构预检、已有文件保护、SQLite 事务、普通失败时文件回滚 | 电脑 SQLite 故障注入通过；Vera 索引、升级、断电恢复待真机验证 |
| 源代码与许可 | AGPL-3.0-only；上游归属、依赖许可、对应源码随包分发 | 构建脚本同时生成 source.tar.gz 与 ARM ZIP；源代码可在界面下载 |

本地导入是阅读核心的测试路径，**不算起点、长佩或 MiuRead 内容接入成功**。没有把上游 Lua 插件放入包内后直接调用，也未把网页跳转冒充 MiuRead 的移植。

## 技术决策与边界

采用设备自带 Mesquite 承载本地 HTML，网络和文件由静态 ARM helper 处理。它绕开旧浏览器在 HTTPS/跨域上的限制，但仍依赖固件的 Mesquite、Scriptlets、appreg 结构和系统证书。Go 只存在于编译端；设备不需要 Python、Node、KUAL 或 KOReader。

对入口机制的证据来自 [Scriptlets](https://kindlemodding.org/kindle-dev/scriptlets/)、[Mesquite 配置](https://kindlemodding.org/wafs-and-mesquite/understanding-config-xml/)、[appmgrd](https://kindlemodding.org/kindle-apps-and-services/com.lab126.appmgrd/) 及已有社区应用的注册方式。它们是选择路线的依据，**不是当前 KPW5 固件上已通过的证明**。没有采用旧 Mesquito Store 缓存加载器。

认证数据留在 helper，前端 JSON 只拿账号名称/ID；源响应以纯文本渲染；本地 API 校验 Host、Origin、随机会话路径、方法及自定义请求头。外部 URL、重定向、响应大小及时间受限制。无无限重试、购买、自动创建 Skills API key 或云进度上传。

每页翻动先写进度；写失败时回到前一页并提示。正常退出进入 Kindle Home 的 LIPC 调用已实现，但真实退出恢复待测试。helper 五分钟无请求自动退出，隐藏页面停止保活；这还不是完整休眠适配，A14 保持未完成。

## 外部阻塞

1. **真实平台网络**：本云环境访问 `www.qidian.com`、`www.gongzicp.com`、`weread.qq.com/api/auth/getLoginUid` 均在代理 CONNECT 阶段返回 403。失败发生在出站策略边界，不能据此认定平台接口失效。已保存六个主机的配置草稿（另含 `book.qidian.com`、`read.qidian.com`、`i.weread.qq.com`），需在环境设置保存并发布，未声称生效。
2. **设备信息/硬件**：尚无完整固件、Vera 版本或可访问测试设备。没有执行真实 Kindle 安装、触控、刷新、休眠、续航或性能测量。
3. **MiuRead 正文**：尚未移植 Web 内容请求上下文、分片、解码、E-Ink 通道。登录和书架不是最小阅读闭环，A21 仍未通过。

## 验证命令及解释

```sh
sh scripts/check.sh        # Go race 测试、vet、Node/ES5 及 diff 检查
npm run test:browser       # 真实本地程序 + Chromium，合成文本交互
sh scripts/build.sh       # ARM/桌面程序、ZIP、对应源码、SHA-256
```

浏览器测试最初发现底部按钮超出视口，加入几何断言后修复；它验证桌面视口内可操作，不代表 Kindle 显示与手势行为已测。Playwright 浏览器 CDN 被环境策略拦截，随后使用环境已有 `/usr/bin/chromium` 完成检查，没有修改网络代理规则。

独立代码复核发现并通过回归修复：多容器目录漏章、被移除章节导致无法从书架进入目录、反复调字号导致位置向前漂移，以及取消后旧二维码回调停止新登录。目录移除的旧位置转为可恢复书签；章节重新出现时可恢复。另按固定上游协议补齐设备指纹保护及 `X-Vid`/`X-Skey` 请求头；这些协议回归不等于真实登录成功。

完整验收仍按 [roadmap](roadmap.md) 的 A00–A23，不能把本表的电脑测试结果填成设备或平台通过。
