# 长佩协议观察与验证边界

日期：2026-10-28。观察官方公开网页及其请求结构，独立实现客户端；没有使用用户账号、没有购买/订阅、没有处理验证码，也没有从上传的微信读书闭源程序抽取逻辑。

| 官网请求 | 本轮实网结果 | 代码 / 合成测试 |
| --- | --- | --- |
| `GET /webapi/search/novels?k=春&page=1` | 返回 10 条书籍元数据 | 查询编码、分页、异常 envelope |
| `GET /webapi/novel/novelInfo?id=2131649` 和 `chapterGetList?nid=2131649` | 目录转换为 64 个章节；分卷行不是章节 | 书籍 ID、章节 ID、权限标记、目录刷新 |
| `GET /webapi/home/initPcIndex` | 提取 5 组有数据的官网榜单 | 按官网分组保留，不填充假数据 |
| `POST /cp.UserExternal/get_qrcode_token` | 创建有效格式的二维码 token | `scene=Login` protobuf、帧长度、错误头、HTTP trailer、超限/受损响应 |
| `POST /cp.UserExternal/qrcode_polling` | 实际等待超过旧 20 秒上限；延长观察后返回 `waiting` | waiting/scanned/approved/rejected/invalid；取消后的迟到响应不能保存账号 |
| `GET /webapi/novel/chapterGetInfo?cid=8675665&server=0` | 响应带 `check=2`，需要官方验证，未作为成功阅读 | 验证、锁章、未购、App 专属、福利章、未知渲染格式均显式拒绝；仅合成授权文字验证解码 |

官方网页资源：

- [首页](https://www.gongzicp.com/)
- [登录页](https://www.gongzicp.com/login/signin)
- [当日入口脚本](https://www.gongzicp.com/assets/index.b20678f0.js)
- [扫码界面脚本](https://www.gongzicp.com/assets/SignInPc.99c69822.js)
- [阅读器脚本](https://www.gongzicp.com/assets/ReadPc.5d771494.js)

这些资源 URL 含构建版本，平台更新后可能变化。程序不会下载或执行这些脚本，协议由 Go 实现。

## 已确认的流程

二维码目的地为官方 `/transfer/login/middleware`，参数含 `token`、`uid`、`pageType=app_qr_code_login`。匿名二维码的 uid 可以为 0；实际账号 ID 由确认后的用户信息接口验证，不使用二维码 uid 作为账号。

RPC 是 gRPC 二进制消息。官网同源响应可用 `application/grpc` 单数据帧和 HTTP trailer；同时兼容 grpc-web trailer 帧。错误必须检查，HTTP 200 不代表成功。元数据包含 version=1 和安装实例随机指纹；不读取 Kindle 序列号，不假冒用户手机设备 ID。

长轮询使用 12 秒本机等待窗口，正常沉默返回 waiting，前端在响应后间隔 2 秒继续；它不是每两秒新增一个并发请求。用户取消会取消当前网络请求。二维码本机最多保留两分钟，之后手动刷新。

成功状态需要 LoginRes.status=1、Web token 和用户信息校验；手机号验证等其他状态提示去官方 App 处理。凭据只保存在 helper；前端 JSON、日志、源码包不含真实 token、二维码内容或小说正文。

正文接口当前接收官方 renderType=1 文本，先检查验证与权益，再按官网公开客户端编码转换为段落块。作者附言保留为 note 块并与正文一起缓存。图片/Canvas 类 renderType=2 明确提示尚未适配，不丢弃后伪装完整章节。

## 尚未证明

真实手机确认、真实已购章节、账号续期字段的端到端行为、官方风控完成后的正文、旧固件 TLS 信任、KPW5 启动/分页/刷新/休眠。协议用例覆盖这些分支的本地行为，不替代实网与实机验收。
