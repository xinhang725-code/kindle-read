# KPW5 / Vera 长佩阅读器技术选择

2026-10-28。用户要求设备低负担、原生书架点开即用、不依赖 KOReader，首次必须扫码。两份上传规格作为界面和接口参考，DeepSeek 的技术方案不是已在设备上验证的事实。

| 方案 | 设备负担与翻页 | 安装与维护 | 判断 |
| --- | --- | --- | --- |
| 直接打开官网 | Vue、脚本、Canvas 和浏览器兼容性负担；网络变化影响页面 | 安装少，旧 WebKit 支持不确定 | 不作为阅读器 |
| 本机 Go helper + 简单 ES5 / Mesquite | 系统已有 WebKit；只有当前章节参与排版，翻页替换当前页，所有网络/编码在 helper | 无自建服务器；可沿用现有 Scriptlet / appmgr 启动 | 当前实现 |
| 本机 C/LVGL 或 FBInk + 本机 helper | 原生布局可能降低内存和绘制负担；需自做断行、字形缓存和墨水屏刷新 | 需适配 framebuffer、触摸坐标、方向、休眠、恢复系统界面 | 保留为实测后的替换前端路线 |
| C/LVGL + 远程后端 | 网络解析负担转移到服务器，但本地排版与刷新仍需做 | 多一个服务器、证书和账号托管依赖；离线更复杂 | 不选作默认方案 |
| 生成 EPUB 后交给系统阅读器 | 系统排版成熟 | 书逐本进系统书架，账号/章节更新/段评体验分裂 | 不满足「App 内看书」目标 |

目前没有实机内存、延迟和耗电数据，因此不声称此路线是所有固件上最快。优先减少实际工作量：无 Vue、无动画、无后台预下载、书城/目录分批显示、正文只绘当前页、字号重排保留逻辑锚点、正常翻页不访问长佩。没有额外打包大字体，使用设备已有中文字体。此路线的关键验收是旧 WebKit 的分页速度与字形表现。

## 原生入口

KPW5 不能直接假定拥有老款 Kindle 的 KDK / Kindlet 运行时。`KUAL_Booklet` 与 `.azw2` Kindlet 也不能混为一谈。文档中的「书架只支持两类文件」没有覆盖 Vera Scriptlets 的启动机制。

当前实现使用 Vera Scriptlet 入口，启动本机 helper，再通过独立 appmgr ID 打开 Mesquite。社区机制资料：[Scriptlets](https://kindlemodding.org/kindle-dev/scriptlets/)、[Mesquite](https://kindlemodding.org/wafs-and-mesquite/understanding-config-xml/)、[appmgrd](https://kindlemodding.org/kindle-apps-and-services/com.lab126.appmgrd/)。具体固件能否索引并启动仍须 KPW5 验收，未把桌面测试写作真机成功。

## 如何使用已有前端设计

保留扫码、书架、书城、目录、沉浸式阅读及设置布局，采用此前黑白 v2 的四灰阶、白底、48px 触控和零动画。最新文档提出的 C 工程和远程 API 是方案草图，附件没有可直接编译的 C/LVGL 源码；参考布局在现有运行时实现。

字体默认 20px、可选 18–26px，行距默认 1.75，边距默认 26px；均不依赖 Android 控件。暂不实现后稿的米黄/夜间背景，维持已确认的白底黑字设计。未做功能不放可点击的假按钮。

## 接口适配

- 本机接口位于随机 loopback 端口的 `/s/<随机会话>/api/`；校验 Host、Origin 和 `X-Kindle-Read: 1`，不开放远程代理。
- `login/start`、`login/poll`、`login/cancel` 完成长佩 App 扫码；前端只取得本机二维码图片路径和状态，长佩 token 留在 helper。重启通过用户信息接口校验，已识别的失效会话尝试续期或重新登录。
- `state` 返回账号和本设备书架；`discover` 返回官网首页现有榜单；`search` 分页搜索；`book` 获取目录并加入本地书架；`chapter` 返回 `blocks:[{t:"p"|"note",text:...}]` 及兼容 `paragraphs`，不返回 HTML；`progress` 保存精确位置。
- 错误包含 `code/message/retryable`，另外保留旧界面的 `error`。新增 `VERIFICATION_REQUIRED`、`CHAPTER_LOCKED`、`APP_REQUIRED`、`UNSUPPORTED_CONTENT`，不把验证/格式问题统称未购买。
- 偏移明确为 **UTF-16 code units**，段号从 0 开始。将来 C 前端需要按同一规则换算，避免 emoji 后位置不一致。
- 原稿 `/v1` 远程 Bearer 接口、云端书架、评论、图片代理尚未实现，不宣称兼容整份 8 组 API。现有结构化正文与账号隔离可供未来原生前端复用。

## 真机比较方法

同一 KPW5/固件/字体/章节，测冷启动、100 次翻页的中位与 P95、首次分页/重排、RSS、退出和休眠恢复。保留电量、温度、网络和刷新方式；不拿桌面 Chromium 数字代替 Kindle。若 Mesquite 的内存或翻页不达标，保留协议/账号/缓存服务，只替换 C 前端，再用相同章节复测。当前没有连接真实 Kindle。
