# wcfLink

Go 核心库 + 本地服务，接入 iLink 个人微信通道和企业微信（WeCom），内置 AI Agent 智能助手。

两种使用方式：

- **作为 Go 库**嵌入到你的程序
- **作为本地 HTTP 服务**独立运行

桌面应用：[wcfLink-GUI](https://github.com/QB-Chen/wcfLink-GUI)

---

## 功能概览

| 模块 | 能力 |
|------|------|
| **iLink 个人微信** | 扫码登录、消息收发（文本/图片/视频/文件/语音）、媒体加解密、输入状态、群消息识别、引用消息、Bot 生命周期管理 |
| **企业微信** | XML 回调验证、入站消息监听、自动回复（webhook/echo）、主动发送、媒体上传、通讯录查询、多账号管理 |
| **AI Agent** | 多轮对话、Function Calling、5 种专业模式、9 个内置工具、长消息分段、双通道自动路由 |
| **客服助手** | 三层架构（Builder/Behavior/Runtime）、多套客服规范切换、知识库、工单系统、订单系统、退款流程、升级策略 |

---

## 快速开始

### 环境要求

- Go `1.25+`
- SQLite（自动管理，无需额外安装）

### 方式一：本地 HTTP 服务

```bash
go build -o ./bin/wcfLink ./cmd/wcfLink
./bin/wcfLink
```

默认监听 `127.0.0.1:17890`，启动后通过 HTTP API 完成扫码登录、查询账号、拉取事件、发送消息。

查看版本：

```bash
./bin/wcfLink -version
```

### 方式二：Go 库嵌入

```bash
go get github.com/QB-Chen/wcfLink@latest
```

```go
package main

import (
    "context"
    "log/slog"
    "os"

    "github.com/QB-Chen/wcfLink/engine"
)

func main() {
    ctx := context.Background()
    cfg := engine.LoadConfig()
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    eng, err := engine.New(ctx, cfg, logger)
    if err != nil {
        panic(err)
    }
    defer eng.Shutdown()

    if err := eng.StartBackground(ctx); err != nil {
        panic(err)
    }
    select {}
}
```

---

## iLink 个人微信通道

### 支持的功能

- 扫码登录 + 登录状态轮询
- 已登录账号持久化
- `getupdates` 长轮询收消息
- 文本消息收发
- 图片、视频、文件发送和接收（AES-ECB 加解密）
- `context_token` 管理（维持消息对话上下文）
- Bot 配置查询（`getconfig`，获取 `typing_ticket`）
- 输入状态指示器（`sendtyping`，对方看到"正在输入..."）
- Bot 生命周期通知（`notifystart` / `notifystop`）
- 群消息 `group_id` 字段（区分群 ID 与发送者 ID）
- 引用消息（`ref_msg`）
- Tool Call 消息类型（type 11/12）

### 登录流程

1. 发起登录 → 获取二维码
2. 轮询登录状态
3. 用户扫码确认
4. 账号自动持久化，启动长轮询

**Go 库示例：**

```go
session, _ := eng.StartLogin(ctx, "")
png, _ := eng.GetLoginQRCodePNG(ctx, session.SessionID)
os.WriteFile("qrcode.png", png, 0o644)

// 轮询状态
status, _ := eng.GetLoginStatus(ctx, session.SessionID)
```

**HTTP API 示例：**

```bash
# 发起登录
curl -X POST http://127.0.0.1:17890/api/accounts/login/start -H 'Content-Type: application/json' -d '{}'

# 轮询状态
curl "http://127.0.0.1:17890/api/accounts/login/status?session_id=login_xxx"

# 获取二维码图片
curl -o qrcode.png "http://127.0.0.1:17890/api/accounts/login/qr?session_id=login_xxx"
```

### 发送消息

```go
// 发送文本（contextToken 传空会自动查找已保存的上下文）
eng.SendText(ctx, accountID, toUserID, "你好", "")

// 发送媒体（支持 image/video/file）
eng.SendMedia(ctx, accountID, toUserID, "image", "/path/demo.jpg", "图片说明", "")
```

```bash
# HTTP 发送文本
curl -X POST http://127.0.0.1:17890/api/messages/send-text \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"xxx@im.bot","to_user_id":"yyy@im.wechat","text":"你好"}'

# HTTP 发送媒体
curl -X POST http://127.0.0.1:17890/api/messages/send-media \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"xxx@im.bot","to_user_id":"yyy@im.wechat","type":"image","file_path":"/path/demo.jpg"}'
```

### Bot 高级功能

```bash
# 获取 typing_ticket
curl -X POST http://127.0.0.1:17890/api/bot/getconfig \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"xxx@im.bot","ilink_user_id":"yyy@im.wechat"}'

# 发送"正在输入"（status: 1=输入中, 2=取消）
curl -X POST http://127.0.0.1:17890/api/bot/sendtyping \
  -H 'Content-Type: application/json' \
  -d '{"account_id":"xxx@im.bot","ilink_user_id":"yyy@im.wechat","typing_ticket":"...","status":1}'

# Bot 生命周期
curl -X POST http://127.0.0.1:17890/api/bot/notifystart -H 'Content-Type: application/json' -d '{"account_id":"xxx@im.bot"}'
curl -X POST http://127.0.0.1:17890/api/bot/notifystop -H 'Content-Type: application/json' -d '{"account_id":"xxx@im.bot"}'
```

### 消息新增字段

| 字段 | 说明 |
|------|------|
| `group_id` | 群 ID（群消息时独立于 `from_user_id`） |
| `session_id` | 会话 ID |
| `run_id` | 运行 ID |
| `update_time_ms` / `delete_time_ms` | 消息更新/删除时间 |
| `ref_msg` | 引用消息（被引用的内容 + 摘要） |
| `msg_id` | 消息项 ID |
| `tool_call_start_item` / `tool_call_result_item` | Tool Call 消息（type 11/12） |

---

## 企业微信（WeCom）通道

### 前置准备

1. 登录[企业微信管理后台](https://work.weixin.qq.com/wework_admin/frame)
2. 创建自建应用，记录 `AgentId` 和 `Secret`
3. 获取 `CorpId`（企业信息页）
4. 配置回调 URL → `http://<你的服务>/api/wecom/callback`，记录 `Token` 和 `EncodingAESKey`

### 配置

```bash
export WCFLINK_WECOM_CORP_ID="ww1234567890"
export WCFLINK_WECOM_CORP_SECRET="your-secret"
export WCFLINK_WECOM_AGENT_ID="1000002"
export WCFLINK_WECOM_CALLBACK_TOKEN="your-token"
export WCFLINK_WECOM_CALLBACK_AES_KEY="your-aes-key"
export WCFLINK_WECOM_AUTO_REPLY="true"
export WCFLINK_WECOM_WEBHOOK_URL="http://your-agent/webhook"  # 可选
```

### 自动回复机制

当 `WCFLINK_WECOM_AUTO_REPLY=true` 时：

1. **Webhook 模式**：将入站消息 POST 到 `WCFLINK_WECOM_WEBHOOK_URL`，将响应中的 `reply` 字段回复给用户
2. **Echo 模式**：未配置 webhook 时，自动回复确认消息

Webhook 请求格式：

```json
{
  "channel": "wecom",
  "corp_id": "ww1234567890",
  "agent_id": 1000002,
  "from_user": "UserName",
  "msg_type": "text",
  "content": "用户发送的消息",
  "msg_id": 123456789,
  "received_at": "2025-01-01T00:00:00Z"
}
```

### HTTP API 管理

```bash
# 添加账号
curl -X POST http://127.0.0.1:17890/api/wecom/accounts \
  -H 'Content-Type: application/json' \
  -d '{"corp_id":"ww123","corp_secret":"xxx","agent_id":1000002,"callback_token":"tk","callback_aes_key":"key","auto_reply":true}'

# 查询账号 / 事件
curl http://127.0.0.1:17890/api/wecom/accounts
curl "http://127.0.0.1:17890/api/wecom/events?after_id=0&limit=100"

# 主动发送
curl -X POST http://127.0.0.1:17890/api/wecom/messages/send-text \
  -H 'Content-Type: application/json' \
  -d '{"corp_id":"ww123","corp_secret":"xxx","agent_id":1000002,"to_user":"UserName","text":"你好"}'

# 通讯录查询
curl "http://127.0.0.1:17890/api/wecom/contacts/user?corp_id=ww123&corp_secret=xxx&user_id=zhangsan"
curl "http://127.0.0.1:17890/api/wecom/contacts/users?corp_id=ww123&corp_secret=xxx&department_id=1"
curl "http://127.0.0.1:17890/api/wecom/contacts/departments?corp_id=ww123&corp_secret=xxx"
curl "http://127.0.0.1:17890/api/wecom/contacts/groupchat?corp_id=ww123&corp_secret=xxx&chat_id=CHATID"
```

### Go 库使用

```go
eng.WeComSendText(ctx, corpID, corpSecret, agentID, "UserName", "你好 <@zhangsan>")
eng.WeComGetUser(ctx, corpID, corpSecret, "zhangsan")
eng.WeComListDepartmentUsers(ctx, corpID, corpSecret, 1)
eng.WeComListDepartments(ctx, corpID, corpSecret)
eng.WeComGetGroupChat(ctx, corpID, corpSecret, "CHATID")
```

---

## AI Agent

### 概述

wcfLink 内置 AI Agent 引擎，将微信 Bot 升级为智能助手。支持多轮对话、工具调用、多种专业模式，通过 LLM（DeepSeek/OpenAI 等）处理用户请求。

### 启用配置

```bash
export WCFLINK_AGENT_ENABLED="true"
export WCFLINK_LLM_BASE_URL="https://api.deepseek.com"    # 支持任何 OpenAI Compatible API
export WCFLINK_LLM_API_KEY="sk-xxx"                        # 必填
export WCFLINK_LLM_MODEL="deepseek-chat"
export WCFLINK_LLM_TEMPERATURE="0.7"
export WCFLINK_LLM_MAX_TOKENS="4096"
export WCFLINK_AGENT_DEFAULT_MODE="icemark"
export WCFLINK_AGENT_MAX_ITERATIONS="10"
export WCFLINK_AGENT_SESSION_TTL="168h"                    # 会话过期时间（默认 7 天）
export WCFLINK_FETCH_MAX_CONTENT_LENGTH="8000"
```

### 工作流程

```
微信用户发消息
  → wcfLink 接收（iLink / WeCom）
  → Agent 引擎：
      1. 加载会话历史（per-user/per-group 隔离）
      2. 拼接 SystemPrompt（基础 SOP + 自定义规范）
      3. 调用 LLM
      4. LLM 决策：
         - tool_calls → 执行工具 → 注入结果 → 继续循环
         - stop → 生成最终回复
  → 自动分段发送回复
```

Agent 启用后，入站文本消息自动路由到 Agent，不再转发到 webhook。未启用时完全不影响现有流程。

### 命令系统

| 命令 | 说明 |
|------|------|
| `/icemark` | 切换到通用助手模式（默认） |
| `/market` | 切换到市场分析模式（SWOT、PESTEL、波特五力） |
| `/prd` | 切换到 PRD 文档模式（用户故事、JTBD） |
| `/prototype` | 切换到原型设计模式（生成 HTML 原型） |
| `/support` | 切换到客服助手模式 |
| `/support-setup` | 启动客服规范配置向导 |
| `/support-profiles` | 查看所有客服规范 |
| `/support-use <名称>` | 切换默认客服规范 |
| `/reset` | 清空当前会话历史 |
| `/mode` | 查看当前模式 |
| `/help` | 显示帮助 |

### 内置工具

| 工具 | 说明 | 可用模式 |
|------|------|----------|
| `web_search` | 多引擎搜索（DuckDuckGo → Bing 降级），支持知乎/小红书/微博 | 全部 |
| `url_content_fetch` | 获取网页内容，转换为纯文本 | 全部 |
| `kb_search` | 搜索知识库（FAQ/产品文档），支持分类过滤 | 客服 |
| `ticket_create` | 创建客服工单 | 客服 |
| `ticket_query` | 查询工单列表或详情 | 客服 |
| `ticket_update` | 更新工单状态/优先级/备注 | 客服 |
| `order_query` | 查询订单信息 | 客服 |
| `order_create` | 创建订单记录 | 客服 |
| `order_refund` | 处理退款（含金额校验和重复退款检查） | 客服 |

### Agent HTTP API

```bash
# 查询 Agent 状态
curl http://127.0.0.1:17890/api/agent/status

# HTTP 直接对话
curl -X POST http://127.0.0.1:17890/api/agent/chat \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"test-1","message":"帮我分析 AI 编程助手的市场趋势"}'

# 会话管理
curl http://127.0.0.1:17890/api/agent/conversations
curl http://127.0.0.1:17890/api/agent/conversations/{id}
curl -X DELETE http://127.0.0.1:17890/api/agent/conversations/{id}
```

---

## 客服助手模式

### 三层架构

```
┌─────────────────────────────────────────────────────┐
│  ① Builder 层（/support-setup）                       │
│  通过多轮对话收集业务信息，动态生成客服规范              │
│  输入：公司名、行业、问候语、退款政策、升级阈值、营业时间 │
│  输出：客服规范 Profile（存入 SQLite）                  │
└────────────────────────┬────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────┐
│  ② Behavior 层（SystemPrompt）                        │
│  定义客服服务规范 SOP                                  │
│  ├── 服务流程：倾听 → 共情 → 确认 → 解决 → 满意确认    │
│  ├── LATTE 投诉模型：Listen → Acknowledge → Take       │
│  │   action → Thank → Explain                         │
│  ├── 语气规范 + 共情话术 + 禁止话术                     │
│  └── 升级策略：法律/高额退款/安全/媒体 → 必须升级       │
└────────────────────────┬────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────┐
│  ③ Runtime 层（Agent 主循环 + 工具）                    │
│  实际执行客服工作                                      │
│  ├── 消息分类 → 知识库搜索 → 工具调用 → 生成回复        │
│  ├── 工具：kb_search / ticket_* / order_*              │
│  └── 升级输出："[需升级] 原因 + 客户信息 + 建议处理"      │
└─────────────────────────────────────────────────────┘
```

### 多套客服规范

支持创建多套不同类型的客服规范，随时切换：

- **预配置默认（方式 A）**：内置 SOP 规范，开箱即用
- **动态生成（方式 B）**：通过 `/support-setup` 多轮对话创建自定义规范

每套规范可自定义：

| 字段 | 说明 | 示例 |
|------|------|------|
| `name` | 规范名称 | 电商客服、SaaS技术支持 |
| `company_name` | 公司/品牌名 | 优品商城 |
| `industry` | 行业类型 | 电商、SaaS、教育、金融 |
| `greeting` | 开场问候语 | "您好！欢迎来到优品商城..." |
| `escalation_threshold` | 升级阈值（元） | 500（超过此金额退款需人工） |
| `refund_policy` | 退款政策 | 7 天无理由退款 |
| `business_hours` | 营业时间 | 周一至周五 9:00-18:00 |
| `extra_config` | 额外业务规则 | 任何自定义规则 |

**使用流程：**

```
/support-setup          → 创建新规范（多轮对话引导）
/support-profiles       → 查看所有已创建的规范
/support-use <名称>     → 将某个规范设为默认
/support                → 进入客服模式（使用当前默认规范）
```

**工作原理：**

```
最终 SystemPrompt = 基础 SOP（Behavior 层）+ 当前默认 Profile 的自定义配置
```

切换规范后，所有后续对话自动使用新规范的公司名、退款政策、升级阈值等。

### 使用示例

**创建客服规范（Builder）：**

```
用户: /support-setup
Bot:  已进入客服规范配置向导。请告诉我你想配置什么类型的客服规范？

用户: 电商客服
Bot:  好的！请问你的公司/品牌名称是什么？

用户: 优品商城
Bot:  收到。退款政策是怎样的？

用户: 7天无理由退货，金额不限
Bot:  明白了。我来确认一下配置...
      客服规范「电商客服」已创建成功！使用 /support 切换到客服模式即可生效。
```

**客服对话（Runtime）：**

```
用户: /support
Bot:  已切换到客服助手模式。

用户: 我买的东西一直没发货，订单号是 ORD-001
Bot:  [调用 order_query 查询订单]
      我理解您着急等快递的心情。我查到您的订单目前状态是"已付款"，
      确实还未发货。我现在就为您创建一个跟踪工单...
      [调用 ticket_create 创建催发货工单]
      工单已创建（ID: abc123），我们会尽快跟进，
      预计 24 小时内给您回复。还有其他问题吗？
```

**切换规范：**

```
用户: /support-profiles
Bot:  共 2 个客服规范配置：
      • 电商客服 ★ 默认 | 公司: 优品商城 | 行业: 电商
      • SaaS技术支持 | 公司: 云软件 | 行业: SaaS

用户: /support-use SaaS技术支持
Bot:  已将「SaaS技术支持」设为默认客服规范。
```

### 客服 HTTP API

**知识库管理：**

```bash
# 添加条目
curl -X POST http://127.0.0.1:17890/api/support/kb \
  -H 'Content-Type: application/json' \
  -d '{"category":"退款","question":"如何申请退款？","answer":"7天内无理由退款，请提供订单号。","tags":"退款,售后","priority":10}'

# 搜索（支持按分类过滤）
curl "http://127.0.0.1:17890/api/support/kb/search?q=退款&category=售后"

# 列出 / 获取 / 更新 / 删除
curl "http://127.0.0.1:17890/api/support/kb?category=退款"
curl http://127.0.0.1:17890/api/support/kb/{id}
curl -X PUT http://127.0.0.1:17890/api/support/kb/{id} -H 'Content-Type: application/json' -d '{"answer":"更新后的回答"}'
curl -X DELETE http://127.0.0.1:17890/api/support/kb/{id}
```

**工单管理：**

```bash
# 创建
curl -X POST http://127.0.0.1:17890/api/support/tickets \
  -H 'Content-Type: application/json' \
  -d '{"subject":"订单未发货","description":"超过3天未发货","customer_id":"user123","priority":"high","category":"物流"}'

# 查询（支持过滤：status/priority/category/customer_id/assignee）
curl "http://127.0.0.1:17890/api/support/tickets?status=open&priority=high"

# 获取详情 / 更新
curl http://127.0.0.1:17890/api/support/tickets/{id}
curl -X PUT http://127.0.0.1:17890/api/support/tickets/{id} \
  -H 'Content-Type: application/json' -d '{"status":"resolved","notes":"已处理"}'
```

**订单管理：**

```bash
# 创建
curl -X POST http://127.0.0.1:17890/api/support/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"user123","product":"年度会员","amount":299.00,"payment_method":"微信支付"}'

# 查询（支持过滤：customer_id/status/product）
curl "http://127.0.0.1:17890/api/support/orders?customer_id=user123"
curl http://127.0.0.1:17890/api/support/orders/{id}

# 退款（校验订单存在性、重复退款、金额超限）
curl -X POST http://127.0.0.1:17890/api/support/orders/{id}/refund \
  -H 'Content-Type: application/json' -d '{"amount":299.00,"reason":"7天无理由退款"}'
```

**客服规范管理：**

```bash
# 创建
curl -X POST http://127.0.0.1:17890/api/support/profiles \
  -H 'Content-Type: application/json' \
  -d '{"name":"电商客服","company_name":"优品商城","industry":"电商","greeting":"您好！欢迎来到优品商城","escalation_threshold":500,"refund_policy":"7天无理由退款","business_hours":"周一至周五 9:00-18:00","is_default":true}'

# 列出 / 获取 / 更新 / 删除
curl http://127.0.0.1:17890/api/support/profiles
curl http://127.0.0.1:17890/api/support/profiles/{id}
curl -X PUT http://127.0.0.1:17890/api/support/profiles/{id} -H 'Content-Type: application/json' -d '{"greeting":"新问候语"}'
curl -X DELETE http://127.0.0.1:17890/api/support/profiles/{id}

# 设为默认
curl -X POST http://127.0.0.1:17890/api/support/profiles/{id}/default
```

---

## 数据模型

所有数据存储在同一个 SQLite 数据库中：

| 表 | 说明 | 索引 |
|----|------|------|
| `accounts` | iLink 已登录账号 | account_id |
| `events` | iLink 入站/出站事件 | id, group_id |
| `login_sessions` | 登录会话 | session_id |
| `peer_contexts` | context_token 缓存 | account_id + user_id |
| `wecom_accounts` | 企业微信账号 | corp_id + agent_id |
| `wecom_events` | 企业微信事件 | id |
| `conversations` | Agent 会话元数据 | channel_type + user_id + group_id (UNIQUE) |
| `conversation_messages` | Agent 消息历史 | conversation_id |
| `tool_call_logs` | 工具调用日志 | conversation_id |
| `kb_articles` | 知识库条目 | category |
| `tickets` | 工单 | status, customer_id |
| `orders` | 订单 | customer_id, status |
| `support_profiles` | 客服规范配置 | name (UNIQUE) |

所有表在首次启动时自动创建。

---

## 完整 HTTP API 列表

### 基础

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/health/live` | 存活检查 |
| GET | `/health/ready` | 就绪检查 |
| GET | `/api/version` | 版本信息 |

### iLink

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/api/accounts/login/start` | 发起登录 |
| GET | `/api/accounts/login/status` | 查询登录状态 |
| GET | `/api/accounts/login/qr` | 获取二维码 PNG |
| GET | `/api/accounts` | 已登录账号列表 |
| GET | `/api/events` | 事件列表 |
| GET | `/api/logs` | 日志列表 |
| GET | `/api/settings` | 获取设置 |
| POST | `/api/settings` | 更新设置 |
| POST | `/api/messages/send-text` | 发送文本 |
| POST | `/api/messages/send-media` | 发送媒体 |
| POST | `/api/bot/getconfig` | 获取 Bot 配置 |
| POST | `/api/bot/sendtyping` | 发送输入状态 |
| POST | `/api/bot/notifystart` | 通知 Bot 启动 |
| POST | `/api/bot/notifystop` | 通知 Bot 停止 |

### 企业微信

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/wecom/accounts` | 账号列表 |
| POST | `/api/wecom/accounts` | 添加账号 |
| DELETE | `/api/wecom/accounts` | 删除账号 |
| GET | `/api/wecom/events` | 事件列表 |
| POST | `/api/wecom/messages/send-text` | 发送文本 |
| GET | `/api/wecom/contacts/user` | 查询用户 |
| GET | `/api/wecom/contacts/users` | 部门成员列表 |
| GET | `/api/wecom/contacts/departments` | 部门列表 |
| GET | `/api/wecom/contacts/groupchat` | 群聊详情 |
| GET/POST | `/api/wecom/callback` | 回调处理 |

### Agent

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/agent/status` | Agent 状态 |
| POST | `/api/agent/chat` | HTTP 直接对话 |
| GET | `/api/agent/conversations` | 会话列表 |
| GET | `/api/agent/conversations/{id}` | 会话详情 + 消息历史 |
| DELETE | `/api/agent/conversations/{id}` | 删除会话 |

### 客服支持

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/api/support/kb` | 知识库列表 |
| GET | `/api/support/kb/search` | 知识库搜索（`?q=&category=`） |
| POST | `/api/support/kb` | 添加知识库条目 |
| GET | `/api/support/kb/{id}` | 条目详情 |
| PUT | `/api/support/kb/{id}` | 更新条目 |
| DELETE | `/api/support/kb/{id}` | 删除条目 |
| GET | `/api/support/tickets` | 工单列表（支持过滤） |
| POST | `/api/support/tickets` | 创建工单 |
| GET | `/api/support/tickets/{id}` | 工单详情 |
| PUT | `/api/support/tickets/{id}` | 更新工单 |
| GET | `/api/support/orders` | 订单列表（支持过滤） |
| POST | `/api/support/orders` | 创建订单 |
| GET | `/api/support/orders/{id}` | 订单详情 |
| POST | `/api/support/orders/{id}/refund` | 订单退款 |
| GET | `/api/support/profiles` | 客服规范列表 |
| POST | `/api/support/profiles` | 创建客服规范 |
| GET | `/api/support/profiles/{id}` | 规范详情 |
| PUT | `/api/support/profiles/{id}` | 更新规范 |
| DELETE | `/api/support/profiles/{id}` | 删除规范 |
| POST | `/api/support/profiles/{id}/default` | 设为默认 |

---

## Go 库公开方法

`engine.Engine` 提供以下方法：

**生命周期：**
- `StartBackground(ctx)` / `Shutdown()`

**iLink：**
- `StartLogin` / `GetLoginStatus` / `GetLoginSession` / `GetLoginQRCodePNG`
- `ListAccounts` / `LogoutAccount`
- `ListEvents` / `ListLogs`
- `SendText` / `SendMedia`
- `GetConfig` / `SendTyping` / `NotifyStart` / `NotifyStop`
- `GetSettings` / `UpdateSettings`

**企业微信：**
- `WeComSendText` / `WeComSendMedia`
- `WeComListAccounts` / `WeComListEvents`
- `WeComAddAccount` / `WeComRemoveAccount`
- `WeComGetUser` / `WeComListDepartmentUsers` / `WeComListDepartments` / `WeComGetGroupChat`

**版本：**
- `engine.CurrentVersion()`

---

## 配置参考

### 基础配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WCFLINK_LISTEN_ADDR` | 监听地址 | `127.0.0.1:17890` |
| `WCFLINK_STATE_DIR` | 数据目录 | `./bin/data/` |
| `WCFLINK_DB_PATH` | 数据库路径 | `<state_dir>/wcfLink.db` |
| `WCFLINK_MEDIA_DIR` | 媒体目录 | `<state_dir>/media/` |
| `WCFLINK_BASE_URL` | iLink 基础 URL | `https://ilinkai.weixin.qq.com` |
| `WCFLINK_CDN_BASE_URL` | CDN 基础 URL | `https://novac2c.cdn.weixin.qq.com/c2c` |
| `WCFLINK_CHANNEL_VERSION` | 协议版本 | `2.0.1` |
| `WCFLINK_POLL_TIMEOUT` | 长轮询超时 | `35s` |
| `WCFLINK_LOG_LEVEL` | 日志级别 | `info` |

### 企业微信配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WCFLINK_WECOM_CORP_ID` | 企业 ID | - |
| `WCFLINK_WECOM_CORP_SECRET` | 应用 Secret | - |
| `WCFLINK_WECOM_AGENT_ID` | 应用 AgentId | `0` |
| `WCFLINK_WECOM_CALLBACK_TOKEN` | 回调 Token | - |
| `WCFLINK_WECOM_CALLBACK_AES_KEY` | 回调 AESKey | - |
| `WCFLINK_WECOM_API_BASE_URL` | API 地址 | `https://qyapi.weixin.qq.com` |
| `WCFLINK_WECOM_AUTO_REPLY` | 启用自动回复 | `false` |
| `WCFLINK_WECOM_WEBHOOK_URL` | Webhook 地址 | - |

### Agent 配置

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WCFLINK_AGENT_ENABLED` | 启用 Agent | `false` |
| `WCFLINK_AGENT_DEFAULT_MODE` | 默认模式 | `icemark` |
| `WCFLINK_AGENT_MAX_ITERATIONS` | 最大工具调用迭代 | `10` |
| `WCFLINK_AGENT_SESSION_TTL` | 会话过期时间 | `168h` |
| `WCFLINK_LLM_BASE_URL` | LLM API 地址 | `https://api.deepseek.com` |
| `WCFLINK_LLM_API_KEY` | LLM API Key | -（必填） |
| `WCFLINK_LLM_MODEL` | 模型名称 | `deepseek-chat` |
| `WCFLINK_LLM_TEMPERATURE` | 生成温度 | `0.7` |
| `WCFLINK_LLM_MAX_TOKENS` | 最大输出 token | `4096` |
| `WCFLINK_FETCH_MAX_CONTENT_LENGTH` | 网页抓取最大字符 | `8000` |

---

## 项目结构

```
cmd/wcfLink/              二进制入口
engine/                   公开 API 门面
internal/
├── app/                  应用核心逻辑
├── httpapi/              HTTP 路由 + 处理器
├── ilink/                iLink 协议实现
├── wecom/                企业微信客户端 + 回调处理 + 加解密
├── agent/                AI Agent 引擎
│   ├── modes/            模式定义（icemark/market/prd/prototype/support）
│   ├── tools/            工具实现（搜索/抓取/知识库/工单/订单）
│   └── support/          客服支持（Builder + Store）
├── llm/                  LLM 客户端（OpenAI Compatible）
├── store/                SQLite 存储层
├── worker/               后台轮询
├── model/                共享类型
└── config/               配置加载
version/                  版本信息
docs/                     设计文档
```
