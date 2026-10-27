# 许可与上游归属

本项目代码使用 **AGPL-3.0-only**，完整文本见 `LICENSE`。

## MiuRead

- 上游：[miumiupy98-art/miuread-koreader](https://github.com/miumiupy98-art/miuread-koreader)
- 固定版本：`581f9b660097aa42fe33170098346862cf0a9bbf`
- 许可：AGPL-3.0-only。
- 修改：将 `auth.lua`、`cookies.lua`、`device_identity.lua`、`api_web.lua` 的 Web 登录、设备身份、稳定 Cookie 边界、书架接口流程移植为 Go（`internal/miuread/auth.go`）。界面、存储、网络和二维码组件已替换，不包含 KOReader。没有移植全部上游功能。
- 本项目中的「MiuRead 独立版」为修改版，不是上游发布的独立程序。上游作者未被声称为本项目背书。
- 原版权声明原文保留于 `third_party/MiuRead.NOTICE`：Copyright (C) 2026 MiuRead contributors；上游起源为 `finlater/weread.koplugin` v0.1.1，Copyright (C) finlater and contributors。各原作者保有其代码版权。
- `third_party/MiuRead.THIRD_PARTY_NOTICES` 保留上游历史通知供追溯。本包未复制该通知所列的 Noto Emoji 字体或 Lucide/Feather 图标，界面使用设备/浏览器自带字体及文本按钮。

## Go 运行时及模块

| 组件 | 固定版本 | 许可 | 用途 |
| --- | --- | --- | --- |
| Go | 1.25.3 | BSD-3-Clause | 编译器、标准库及静态运行时 |
| golang.org/x/net | v0.46.0 | BSD-3-Clause | HTML 解析 |
| github.com/skip2/go-qrcode | v0.0.0-20200617195104-da1b6568686e | MIT | 本地生成登录二维码 |

测试 ZIP 的 `extensions/kindle-read/third_party/` 包含上述许可；对应源代码包包含本项目、固定依赖源码 `vendor/`、构建脚本和锁文件。界面底部提供同一份源代码下载。Go 源码可在 [官方版本归档](https://go.dev/dl/go1.25.3.src.tar.gz) 获取。

仅开发测试使用的 Playwright（Apache-2.0）、Acorn（MIT）版本由 `package-lock.json` 固定，其包及许可由 `npm ci` 获取，不装入 Kindle 应用。

## 运行机制参考

Scriptlets、Mesquite 配置、appmgr 的资料来自 [KindleModding 文档](https://kindlemodding.org/)；实际社区应用的同类 appreg 注册机制用于可行性调查。没有复制或捆绑这些应用、Mesquito 或系统固件。Mesquite 是设备自带组件，系统组件并不包含在安装包里。
