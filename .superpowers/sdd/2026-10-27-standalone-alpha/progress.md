Plan: docs/superpowers/plans/2026-10-27-standalone-alpha.md

- 2026-10-27：开始 inline 实施。基线 54d0faa，分支 work，工作区干净。
- 技术调查：Vera Scriptlets / Mesquite / appmgr 有社区文档及现有应用机制证据；KPW5 固件、实机可用性未确认。Go 工具链 1.25.3 下载并校验官方 SHA-256；系统 `/usr/bin/go` 不是 Go 编译器，不能使用它。
- 网络：三个内容平台经当前代理均返回 CONNECT 403；六个所需主机已保存到环境配置草稿，requires_publish=true，尚未应用。
- 任务 1 开始：先写持久化及分页行为测试。
- 任务 1：已观察 Go 缺少类型和 Node 缺少模块的失败，再实现来源/账号隔离、原子写入与 DOM 测量分页。后续补测发现 Book slice 可由调用者改写，修复为边界复制。
- 任务 2：先失败后实现受限 HTTPS、公开页面合成解析夹具、MiuRead 真实协议调用的 HTTP 边界测试；二维码/会话/书架的失败、取消、过期和保存测试通过。真实平台访问仍阻塞。
- 任务 3：真实本地 HTTP 集成与 Chromium 交互通过。截图发现底部按钮超屏；新增几何断言观察失败，改为兼容旧浏览器的固定比例布局。补齐阅读中的目录入口。
- 任务 4：安装冲突/故障/卸载测试、真实 SQLite 事务故障注入、进程锁测试通过。静态 linux/arm GOARM=7 ELF 已构建；USB ZIP 与源码/许可随包生成。普通错误回滚已测，断电与真机未测。
- 全量检查：Go race/vet、4 个 Node 测试、2 个浏览器端到端测试通过；浏览器使用已有 /usr/bin/chromium。setup.sh 已实际执行，Go/npm 依赖安装通过。
- 云配置：安装脚本与开始说明已保存为草稿（requires_publish=true），与域名草稿一同待用户在环境设置保存/发布；未阻塞独立开发。
- 提交策略：初始实现的存储、来源、服务和设备接口互相依赖，作为一个可运行 alpha 提交，再按最终 reviewer 结果单独提交修复。没有调用技能的本地辅助脚本，账本手工记录。
- 任务 5 开始：按 requesting-code-review / executing-plans 请求最终独立只读复核。设备/平台未验收不被登记为通过。
