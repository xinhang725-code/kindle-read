# kindle-read

面向已越狱 Kindle 的起点、长佩文学与 MiuRead 独立应用项目。

目标体验：Kindle 原生书架上显示「起点阅读」「长佩阅读」「MiuRead」三个入口，点开即可进入各自阅读流程；有记录时优先续读，无记录时进入对应应用书架或登录流程。名称暂定，修改版 MiuRead 的分发名称及上游归属另行标明，不冒充官方版本。

**当前开发重点：长佩阅读 0.2.0-alpha.1。** 起点和微信读书工作暂停。本轮加入强制长佩 App 扫码登录、官网榜单/搜索/目录、受权限检查的文字章节、黑白分页界面，以及只注册长佩的安装包。真实账号确认和 KPW5/Vera 实机验收尚未完成，仍是开发测试版。

- [长佩包安装与使用](docs/changpei-installation.md)
- [低性能设备方案比较与选择](docs/changpei-architecture.md)
- [长佩协议观察与验证边界](docs/changpei-protocol-evidence.md)

## 开始开发

```sh
sh scripts/setup.sh
sh scripts/check.sh
npm run test:browser
sh scripts/build.sh
```

本地运行：`sh scripts/go.sh run ./cmd/kindle-read serve --source changpei --data .local-data`。将 `changpei` 改为 `qidian` 或 `miuread` 可打开此前的实验入口。网络、文件与运行时由独立 Go 程序处理，界面为适配 Mesquite 的 ES5；安装包不包含 KOReader。

- [实现状态与验证边界](docs/implementation-status.md)
- [MiuRead 依赖说明与新独立版对照任务](docs/2026-10-28-weread-comparison.md)
- [用户提供的微信读书 0.3.6-rc.4：包检查、功能差距与补齐条件](docs/weread-0.3.6-rc.4-audit.md)
- [构建、测试包安装与卸载](docs/installation.md)
- [许可和上游归属](THIRD_PARTY_NOTICES.md)

产物在 `dist/`：ARM 开发测试 ZIP、对应源代码包及 SHA-256。自有文本导入只用于验证阅读核心，不计作平台阅读功能完成。

## 项目文档

- [需求与技术设计](docs/superpowers/specs/2026-10-27-kindle-qidian-design.md)：产品目标、交互流程、方案比较、建议架构与范围。
- [可行性验证与资料](docs/feasibility.md)：原生书架入口、三个内容平台的访问、设备兼容性与已查证资料。
- [MiuRead 独立化方案](docs/miuread-porting.md)：上游功能、KOReader 依赖替换、原生入口及移植验收。
- [里程碑与验收](docs/roadmap.md)：按验证结果推进的开发阶段、验收用例与交付标准。

## 已确认的技术约束

用户明确的目标：**在使用 Vera 越狱的 KPW5 上，从原生书架点开即可使用的阅读 app，不依赖 KOReader**。原先推荐的 KOReader 插件方案已撤下；后台捆绑或隐藏 KOReader 界面不满足此约束。

用户已确认所指项目为 [MiuRead / miuread-koreader](https://github.com/miumiupy98-art/miuread-koreader)，并明确要求 **MiuRead 本身也有原生书架入口，点击即用**。因此它是独立交付项，保留其微信读书用途，不再仅作为交互参考；起点与长佩目标同时保留。现有插件依赖 KOReader，需要评估和实施独立化移植，不能只加一个启动快捷方式就宣称完成。

独立应用的启动、排版、墨水屏和休眠能力需要分别实现或选用合适组件。产品实现前先做最小验证：

1. 独立应用能够在不安装、不加载、不捆绑 KOReader 运行时的条件下运行。
2. 点击原生书架里的项目入口，能直接进入指定阅读界面，并正常退出回到系统。
3. 分别验证起点、长佩以及 MiuRead 对应的微信读书内容；MiuRead 还需验证移植后的登录、账号书架、下载和阅读闭环，不能用另两个应用通过验收代替。

上述要求不能用隐藏依赖、模拟数据、普通电子书导入或多级启动菜单替代验收。

首轮验证设备为 **KPW5（Kindle Paperwhite 5），使用 Vera 越狱**，尚未完成实机验证。完整固件版本、Vera 版本、可用启动方式和各平台付费内容支持范围仍需补齐。MiuRead 独立化是本项目工作，用户不需要预先安装上游 KOReader 插件。

## 三个应用入口

起点阅读、长佩阅读和 MiuRead 都是开发目标。优先评估共用独立阅读基础，账号、内容权限、缓存和进度按起点、长佩、微信读书三个来源隔离。MiuRead 的移植与功能对照单独记录；三个入口各自完成真机验收后，才算整体目标完成。
