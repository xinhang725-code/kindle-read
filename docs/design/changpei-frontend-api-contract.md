# 长佩 Kindle App · 前端规格与接口契约

> **形态**：书架里一个「长佩」App，点进去 = 完整阅读器；**书不进书架，App 进书架**
> **分工**：前端（Kindle C/LVGL）由本规格定义；**后端由 GPT-6 Astra 实现，按 §5 契约交付**
> **UI 基准**：1:1 复刻微信读书（见《微信读书-Android-前端复刻规格书.md》）
> **版本**：2026-10-28

---

## 0. 需求锁定

| 项 | 结论 |
|---|---|
| App 位置 | **Kindle 原生书架**（不是 KUAL 二级菜单） |
| 书的位置 | 不落书架，全在 App 内 |
| 阅读器 | App 自带（核心工作量） |
| 登录 | 首次启动扫码 |
| 前端 | C + LVGL + FreeType |
| 后端 | 由 Astra 提供 HTTP API，前端只跟它说话 |

---

## 1. 「App 进书架」怎么做

### 1.1 原理：Kindlet（.azw2）

Kindle 书架只索引两类东西：亚马逊内容、以及 **Kindlet（Active Content，`.azw2`）**。

```
documents/changpei.azw2        ← 放进这里，书架自动出现
        │
        └─ 点开 → Java 壳（KDK）启动
                    └─ Runtime.exec("/mnt/us/extensions/changpei/start.sh")
                            └─ 启动我们的 C 程序（LVGL 全屏）
                                    └─ 退出 → 回到 Kindle 书架
```

这就是 **KUAL 的工作方式**（`KUAL-KDK-2.0.azw2`）。我们不需要重造，只需要：

1. **Fork `coplate/KUAL_Booklet`**（开源）→ 改包名、应用名、图标
2. 把它的"显示 KUAL 菜单"改成"直接 exec 一个脚本"
3. 脚本内容：`/mnt/us/extensions/changpei/bin/changpei`

### 1.2 目录结构

```
/mnt/us/
├── documents/
│   └── changpei.azw2            ← 书架图标（Kindlet 壳）
└── extensions/
    └── changpei/
        ├── start.sh             ← 启动脚本（chmod +x）
        ├── bin/changpei         ← 我们的 C 程序（arm32 静态或带 .so）
        ├── res/
        │   ├── fonts/NotoSerifSC-subset.ttf
        │   └── icons/*.bin
        └── data/                ← SQLite、缓存、token
```

### 1.3 书架上显示长什么样

- 名字：**长佩**
- 封面：Kindlet 自带一张封面图（自制，黑白，240×320），建议做成"长佩"两字 + 极简线条
- 作者：留空或写 App
- ⚠️ Kindlet 在书架上是"电子书"样式，**无法做成 App 图标样式**，这是系统层面的限制

### 1.4 最大风险：签名

| 风险 | 说明 | 应对 |
|---|---|---|
| Kindlet 需要签名 | 未签名会报 `The title is not signed by a registered developer` | 越狱后可用社区方案重签（KUAL 本身即如此分发）；需在你的机型+固件上实测一次 |
| 新固件可能收紧 | 5.17–5.20 属新版本 | **M0 必须先验证"改过的 azw2 能否上架并启动"**，这一步不通，整个方案要退回 KUAL 二级菜单 |
| KDK 已停更 | Java 壳很薄，只做 exec | 别在壳里写业务逻辑 |

> **M0 的第一件事不是画界面，是做一个"点开 → 屏幕变黑 → 退出"的最小 azw2。** 这一步成功率决定了整个项目的形态。

---

## 2. 架构

```
┌──────────────────────────────────────────┐
│ Kindle 原生书架                            │
│   [长佩] ← .azw2 Kindlet 壳                │
└───────────────┬──────────────────────────┘
                │ exec start.sh
┌───────────────▼──────────────────────────┐
│ C 程序（LVGL）                             │
│  ui/     11 个界面                         │
│  core/   api_client / http / json / db    │
│  res/    fonts / icons                    │
└───────────────┬──────────────────────────┘
                │ HTTPS + Bearer Token
┌───────────────▼──────────────────────────┐
│ Astra 后端                                 │
│  代理长佩 → 结构化 JSON（关键！见 §5.4）    │
└───────────────┬──────────────────────────┘
                │
          长佩 gongzicp.com
```

**前端不做的事**：不解析 HTML、不处理防盗链、不管风控、不做登录协议。
**后端做全部脏活**，前端只消费稳定的 JSON。

---

## 3. 界面清单（11 屏）

| # | 屏 | 对应微信读书 | Kindle 改造点 |
|---|---|---|---|
| 1 | 启动/加载 | S01 | 白底 + 黑图标 + 静态"加载中…"（禁 spinner） |
| 2 | **扫码登录** | S02 | 二维码占屏宽 55%，2s 轮询，超时可刷新 |
| 3 | 书架/追更 | S03 | 4 列网格；封面灰度；未读角标 |
| 4 | 书城（榜单/分类） | S06/S07 | 删搜索框；Tab 用文字+下划线 |
| 5 | 书籍详情 | S08 | 推荐值→追更数/字数/更新时间 |
| 6 | 目录 | S14 | 当前章黑底白字块；右侧"更新于 X" |
| 7 | **阅读器** | S09 | 核心，见 §4 |
| 8 | 阅读设置面板 | S11/S12 | 字号/行距/边距 + 4 背景（黑白灰米黄） |
| 9 | 段评面板 | S10 | 底部弹出，引用段落 3 行 |
| 10 | 我的/账户 | S17/S18 | 书币、阅读时长、设置入口 |
| 11 | 设置 | S16 | 账号、缓存、关于、退出登录 |

**删除**：笔记页、微信文章 ×2、导入弹窗（改为"刷新书架"）。

---

## 4. 阅读器（最高优先级）

### 4.1 分页算法（Kindle 上必须自己算）

```
1. 取章节 blocks[]（后端已结构化）
2. 用 FreeType 逐字测量宽度 → 按容器宽度断行 → 得到行数组
3. 按容器高度切页 → pages[][]
4. 只渲染当前页
```

- **禁止用滚动容器**（墨水屏连续重绘会闪 + 残影）
- 换字号/行距/边距 → 重新分页，并**按 (段落号, 段内字符偏移) 恢复位置**
- 中文避头尾：`。，、；：？！）》」』` 不得居行首

### 4.2 版面参数（对应微信读书「字体设置」四维）

| 项 | 范围 | 默认 |
|---|---|---|
| 字号 | 18 / 20 / 22 / 24 / 26 | 21 |
| 行距 | 1.6 / 1.75 / 1.9 | 1.75 |
| 边距 | 窄/中/宽（20/26/32px） | 中 |
| 首行缩进 | 开/关（2em） | 开 |
| 段间距 | 0.5em | 固定 |

### 4.3 交互

- 三热区：左 1/3 上一页、中 1/3 呼出工具栏、右 1/3 下一页
- 顶部工具栏（48px）：返回、目录、段评开关、下载
- 底部工具栏（56px）：目录、字体、进度、夜间（仅反色）、更多
- **零动画**：切页直接替换，无过渡

### 4.4 刷新策略

| 场景 | 策略 |
|---|---|
| 翻页 | 局刷；每 6 页全刷一次 |
| 改字号/切章 | 全刷 |
| 面板弹出 | 局刷（纯白不透明覆盖） |
| 图片页 | 全刷 |

### 4.5 灰阶

只用 5 档：`#000 / #555 / #767676 / #DDD / #FFF`（对齐 0x11 档位）。
文字下限 `#767676`（4.54:1）。封面灰度后做 Floyd-Steinberg 抖动。

---

## 5. 接口契约（交给 Astra）

### 5.1 通用约定

```
Base:      https://<your-host>/v1
Auth:      Authorization: Bearer <token>
Encoding:  UTF-8 JSON
超时建议:   连接 5s / 读取 15s
错误体:     {"code":"AUTH_EXPIRED","message":"登录已过期","retryable":false}
```

错误码表：

| code | 含义 | 前端行为 |
|---|---|---|
| `AUTH_REQUIRED` | 未登录 | 跳登录页 |
| `AUTH_EXPIRED` | token 失效 | 清 token，跳登录页 |
| `NOT_PURCHASED` | 章节需购买 | 提示"该章需购买"，不重试 |
| `RATE_LIMITED` | 触发限频 | 显示"稍后再试"，冷却 60s |
| `UPSTREAM_ERROR` | 长佩侧异常 | 提示"服务繁忙"，可重试 |
| `NOT_FOUND` | 资源不存在 | 提示并返回上一页 |

### 5.2 登录（扫码）

**① 创建二维码**
```http
POST /auth/qr
→ 200
{
  "qr_id": "a1b2c3",
  "qr_content": "https://gongzicp.com/scan/xxxx",   // 编码进二维码的字符串
  "expires_in": 120
}
```

**② 轮询状态**（前端每 2s 一次）
```http
GET /auth/qr/a1b2c3
→ 200
{
  "status": "waiting",        // waiting | scanned | confirmed | expired
  "token": null,
  "user": null
}
→ 200  (confirmed 时)
{
  "status": "confirmed",
  "token": "eyJhbGciOi...",
  "user": { "uid": "10086", "nickname": "书友265", "avatar": "https://..." }
}
```

**③ 校验/续期**
```http
GET /me
→ 200 { "uid":"10086", "nickname":"...", "coin":0, "vip":false }
```

**④ 退出**
```http
POST /auth/logout   → 204
```

### 5.3 书架与书城

```http
GET /shelf?page=1&size=20
→ 200 {
  "items": [{
    "book_id":"12345",
    "title":"以皇帝特权改变这个世界",
    "author":"就吃开饭",
    "cover":"https://<host>/img?id=12345&w=300&gray=1",
    "last_chapter_id":"888001",
    "last_chapter_title":"第42章 雨夜",
    "last_read_chapter_id":"888000",
    "unread": 3,
    "updated_at": 1793150000
  }],
  "total": 1,
  "updated_today": 1
}

GET /store/rank?type=hot&page=1&size=20      → { "items":[...], "total":97 }
GET /store/categories                        → { "items":[{"id":"1","name":"都市"}] }
GET /store/category/1?page=1&size=20         → { "items":[...] }
```

### 5.4 书籍与目录

```http
GET /books/12345
→ 200 {
  "book_id":"12345", "title":"...", "author":"...",
  "cover":"https://<host>/img?id=12345&w=400&gray=1",
  "intro":"本书通过对经典文献的深入解读…",
  "status":"serializing",        // serializing | finished
  "word_count": 324000,
  "chapter_count": 730,
  "updated_at": 1793150000,
  "in_shelf": true
}

GET /books/12345/toc?page=1&size=100
→ 200 {
  "items":[{
    "chapter_id":"888001", "idx":42, "title":"第42章 雨夜",
    "updated_at":1793150000, "free":true, "locked":false
  }],
  "total":730
}
```

### 5.5 ★ 章节正文（最重要的一个设计）

**后端不返回裸 HTML，返回结构化 blocks。**

原因：Kindle C 端没有 HTML 解析器，也没有能力清洗广告/防盗链/兼容畸形标签。**让后端把脏活干完**，前端只做排版。

```http
GET /chapters/12345/888001
→ 200 {
  "chapter_id":"888001",
  "title":"第42章 雨夜",
  "prev_id":"888000",
  "next_id":"888002",
  "blocks":[
    { "t":"p", "text":"泰瑞尔大公国，白河行省，奥瑞利安男爵领。", "cmt":77 },
    { "t":"p", "text":"微风吹拂，金色的阳光透过庄园书房的窗口…", "cmt":2 },
    { "t":"img", "src":"https://<host>/img?id=ch888001_1&w=800&gray=1",
      "w":800, "h":520 },
    { "t":"note", "text":"求月票，感谢各位的支持。" }
  ]
}
```

**block 类型定义**

| t | 字段 | 渲染 |
|---|---|---|
| `p` | `text`, `cmt?` | 正文段落；`cmt` 为段评数，>0 时在段末画小气泡 |
| `h` | `text` | 小标题（少见） |
| `img` | `src`, `w`, `h` | 插图；w/h 用于预留空间，避免重排 |
| `note` | `text` | 作者有话说；左侧 3px 黑竖线，无底色 |
| `sep` | — | 分隔（如"本章完"） |

**后端需要的处理**：
1. 抓取长佩章节 HTML → 抽取正文容器
2. 去广告、去推荐位、去无关脚本
3. `<p>` 归一化，保留 `<em>`/`<strong>`（可选加 `bold:true` 字段）
4. 图片下载 → 转灰度 → 输出为 `<host>/img?id=...&gray=1`（**顺带解决防盗链**）
5. 段评数若可获取，填进 `cmt`
6. 权限：非免费且未购买 → 返回 `NOT_PURCHASED`

### 5.6 进度

```http
PUT /progress/12345
body: { "chapter_id":"888001", "para":3, "offset":12, "percent":0.031 }
→ 204

GET /progress/12345
→ 200 { "chapter_id":"888001", "para":3, "offset":12, "percent":0.031, "updated_at":1793150000 }
```

> `para + offset` 是精确锚点（段落索引 + 段内字符数），比 percent 稳：换字号重整页后仍能定位。

### 5.7 段评

```http
GET /comments?book_id=12345&chapter_id=888001&para=1&page=1&size=30
→ 200 {
  "para_text":"泰瑞尔大公国，白河行省，奥瑞利安男爵领。",
  "items":[{
    "id":"c1", "user":"路人甲", "avatar":"...",
    "content":"这里的地理设定很有意思", "likes":12, "created_at":1793100000
  }],
  "total":77
}

POST /comments
body: { "book_id":"12345", "chapter_id":"888001", "para":1, "content":"..." }
→ 201 { "id":"c9" }
```

### 5.8 图片代理（强烈建议）

```http
GET /img?id=<any>&w=300&gray=1
→ image/jpeg（灰度、最长边缩放到 w）
```

作用：
- 绕过长佩图片防盗链（后端带 Referer 抓取）
- 统一转灰度（Kindle 只需解码显示，不用做色彩转换）
- 控制体积（300px 封面约 15–25KB）

---

## 6. 前端工程结构

```
changpei/
├── Makefile                    arm-linux-gnueabihf 交叉编译
├── main.c
├── ui/
│   ├── lv_port_disp.c          fb 初始化 + flush_cb（控制全刷/局刷）
│   ├── lv_port_indev.c         触摸
│   ├── theme.c                 灰阶/字号/间距 token
│   ├── scr_splash.c  scr_login.c  scr_shelf.c
│   ├── scr_store.c   scr_detail.c scr_toc.c
│   ├── scr_reader.c  scr_settings.c scr_comments.c
│   └── scr_me.c
├── core/
│   ├── http.c       libcurl + mbedTLS
│   ├── json.c       cJSON 封装
│   ├── api.c        ★ 只调后端，不碰长佩
│   ├── auth.c       token 持久化
│   ├── db.c         SQLite：缓存书/章/进度
│   ├── paginate.c   ★ 分页算法
│   ├── qr.c         qrcodegen（画二维码）
│   └── grayscale.c  图片转灰度 + 抖动
└── res/
    ├── fonts/NotoSerifSC-subset.ttf
    └── icons/*.bin
```

---

## 7. 分期

| 阶段 | 内容 | 判定标准 |
|---|---|---|
| **M0 生死线** | 自制 azw2 上架 + 点开 exec C 程序 + fb 出图 + 触摸 + 中文 | 屏幕上能看到"长佩"并退出回到书架 |
| M1 UI 骨架 | LVGL 移植 + 4 屏静态（登录/书架/详情/阅读器） | 能翻页看假数据 |
| M2 接口联调 | 接 Astra 的 5.1–5.5 | 真书能读 |
| M3 完整 | 段评、设置、进度、缓存、错误处理 | 日常可用 |
| M4 打磨 | 刷新策略、低内存、图标、封面 | 观感接近微信读书 |

**M0 不通就止损**：退回 KUAL 二级菜单（不需要签名，代价是多点一次）。

---

## 8. 风险

| 风险 | 影响 | 应对 |
|---|---|---|
| azw2 签名/上架失败 | **方案形态作废** | M0 先验；退路是 KUAL 扩展 |
| KDK 在新固件被移除 | 同上 | 同上 |
| 中文输入法 | 搜索做不了 | 砍掉搜索，只做浏览 |
| 内存（~512MB） | LVGL 缓冲不足 | 单缓冲 + 局部刷新；封面按需解码 |
| 长佩风控 | 后端抓不到 | 后端串行 + 限频；只取已购章节 |
| 后端未就绪 | 前端无法联调 | 前端先用 mock JSON（按 §5 的 schema 造假数据） |

---

## 9. 给 Astra 的一句话

> 你只需要实现 §5 的 8 组接口，**核心是 `/chapters/...` 必须返回结构化 `blocks` 数组而不是 HTML**，并把图片统一走 `/img` 代理转灰度。前端是 C 写的，没有 HTML 解析能力，这一点决定了整个项目的复杂度。

---

## 10. 验收

- [ ] 书架出现「长佩」，点开进 App，返回回到书架
- [ ] 首启扫码 → 手机确认 → 进书架列表
- [ ] 书架拉到真实数据，封面显示为灰度
- [ ] 打开书 → 目录 → 点章节 → 正文正确排版（无乱码/断行正确/标点不居首）
- [ ] 翻页无残影；改字号后自动重排并保持位置
- [ ] 段评数显示，点开能看到评论
- [ ] 杀进程重进，回到上次位置
- [ ] 断网有明确提示，不白屏
