# wcfLink

`wcfLink` 是一个可复用的 Go 核心库，用来接入 iLink 微信通道和企业微信（WeCom），内置 AI Agent 功能。

它提供两种使用方式：

- 作为 Go 库嵌入到你自己的程序里
- 作为一个本地 HTTP 服务独立运行

桌面应用已经拆分到独立项目 [wcfLink-GUI](https://github.com/QB-Chen/wcfLink-GUI)。

## 当前支持

### iLink 个人微信通道

- 扫码登录
- 登录状态轮询
- 已登录账号持久化
- iLink `getupdates` 长轮询
- 文本消息收发
- 图片、视频、文件发送
- 图片、语音、视频、文件接收与落盘
- 本地事件存储
- `context_token` 管理
- Bot 配置查询（`getconfig`，获取 typing_ticket）
- 输入状态指示器（`sendtyping`，显示"对方正在输入..."）
- Bot 生命周期通知（`notifystart` / `notifystop`）
- 群消息 `group_id` 字段支持（区分群 ID 与发送者 ID）
- 引用消息（`ref_msg`）支持
- Tool Call 消息类型支持（type 11/12）
- 本地 HTTP API
- SQLite 状态存储

### 企业微信（WeCom）通道

- 企业微信自建应用 Agent 模式（XML 回调）
- 回调 URL 验证（签名校验 + AES 解密）
- 入站消息自动监听（文本、图片、语音、视频、文件、事件）
- 自动回复（webhook 转发 + 回复中继，或内置 echo 回复）
- 主动发送文本消息
- 媒体上传与发送（图片、语音、视频、文件）
- 多企业微信账号管理
- 企业微信事件存储
- 通讯录查询（用户信息、部门列表、部门成员、群聊详情）
- Access Token 自动缓存

### AI Agent（内置智能助手）

- 多轮对话（基于 LLM Function Calling）
- 会话管理（per-user / per-group 隔离，SQLite 持久化）
- 内置工具：网页搜索（DuckDuckGo/Bing 多引擎降级）、网页内容抓取、知识库搜索、工单管理、订单管理
- 五种专业模式：通用助手（Icemark）、市场分析（Market）、PRD 文档、原型设计（Prototype）、客服助手（Support）
- 客服支持：三层架构（Builder 生成 → Behavior 规范 → Runtime 执行），LATTE 投诉模型，升级策略
- 客服规范配置向导（Builder）：通过多轮对话动态生成客服规范
- 多套客服规范切换：支持预配置默认 + 用户自定义，任意切换默认规范
- 工单系统：创建、查询、更新、关闭工单，支持优先级/状态/分类
- 订单系统：创建、查询订单，退款处理
- 知识库：FAQ 和产品文档管理，Agent 自动搜索知识库回答问题
- 命令系统：`/icemark`、`/market`、`/prd`、`/prototype`、`/support`、`/support-setup`、`/support-profiles`、`/support-use`、`/reset`、`/mode`、`/help`
- 长消息自动分段发送
- 支持 OpenAI Compatible API（DeepSeek、OpenAI、阿里云、火山引擎等）
- iLink 和企业微信双通道自动路由
- HTTP API 直接对话接口

## 目录

- 公开入口：[engine/engine.go](./engine/engine.go)
- 后台入口：[cmd/wcfLink/main.go](./cmd/wcfLink/main.go)
- 应用服务：[internal/app/app.go](./internal/app/app.go)
- iLink 协议：[internal/ilink/client.go](./internal/ilink/client.go)
- iLink 媒体：[internal/ilink/media.go](./internal/ilink/media.go)
- 企业微信客户端：[internal/wecom/client.go](./internal/wecom/client.go)
- 企业微信回调处理：[internal/wecom/handler.go](./internal/wecom/handler.go)
- 企业微信加解密：[internal/wecom/crypto.go](./internal/wecom/crypto.go)
- AI Agent 引擎：[internal/agent/agent.go](./internal/agent/agent.go)
- LLM 客户端：[internal/llm/client.go](./internal/llm/client.go)
- Agent 工具：[internal/agent/tools/](./internal/agent/tools/)
- Agent 模式：[internal/agent/modes/](./internal/agent/modes/)
- 客服支持模块：[internal/agent/support/](./internal/agent/support/)
- 存储层：[internal/store/store.go](./internal/store/store.go)
- HTTP API：[internal/httpapi/server.go](./internal/httpapi/server.go)
- 轮询 worker：[internal/worker/poller.go](./internal/worker/poller.go)

## 运行要求

- Go `1.25+`
- 默认使用 SQLite

## 快速开始

### 方式一：作为本地 HTTP 服务运行

构建并启动：

```bash
go build -o ./bin/wcfLink ./cmd/wcfLink
./bin/wcfLink
```

默认监听地址：

```text
127.0.0.1:17890
```

启动后你可以通过 HTTP API 完成扫码登录、查询账号、拉取事件、发送消息。

查看当前二进制版本：

```bash
./bin/wcfLink -version
```

### 方式二：作为 Go 库嵌入

先安装模块：

```bash
go get github.com/QB-Chen/wcfLink@latest
```

最小示例：

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

## 登录流程

无论你是通过 Go 库还是 HTTP API，登录流程都一样：

1. 发起登录，拿到二维码会话
2. 轮询登录状态
3. 用户扫码确认
4. 登录成功后账号会自动持久化，并启动长轮询

### Go 库登录示例

```go
session, err := eng.StartLogin(ctx, "")
if err != nil {
	return err
}

png, err := eng.GetLoginQRCodePNG(ctx, session.SessionID)
if err != nil {
	return err
}

_ = os.WriteFile("qrcode.png", png, 0o644)

status, err := eng.GetLoginStatus(ctx, session.SessionID)
if err != nil {
	return err
}

_ = status
```

### HTTP 登录示例

发起登录：

```bash
curl -s -X POST http://127.0.0.1:17890/api/accounts/login/start \
  -H 'Content-Type: application/json' \
  -d '{}'
```

返回里会包含：

- `session_id`
- `qr_code_url`

轮询登录状态：

```bash
curl -s "http://127.0.0.1:17890/api/accounts/login/status?session_id=login_xxx"
```

如果你要直接拿二维码 PNG：

```bash
curl -o qrcode.png "http://127.0.0.1:17890/api/accounts/login/qr?session_id=login_xxx"
```

## 作为库时可直接调用的接口

当前 `engine.Engine` 已公开这些核心方法：

- `StartBackground`
- `Shutdown`
- `StartLogin`
- `GetLoginStatus`
- `GetLoginSession`
- `GetLoginQRCodePNG`
- `ListAccounts`
- `ListEvents`
- `GetSettings`
- `UpdateSettings`
- `SendText`
- `SendMedia`
- `LogoutAccount`
- `GetConfig`
- `SendTyping`
- `NotifyStart`
- `NotifyStop`
- `WeComSendText`
- `WeComSendMedia`
- `WeComListAccounts`
- `WeComListEvents`
- `WeComAddAccount`
- `WeComRemoveAccount`
- `WeComGetUser`
- `WeComListDepartmentUsers`
- `WeComListDepartments`
- `WeComGetGroupChat`

当前公开的版本接口：

- `engine.CurrentVersion()`

### 发送文本

```go
err := eng.SendText(ctx, accountID, toUserID, "你好", "")
```

说明：

- 如果 `contextToken` 传空，会尝试从本地已保存的会话上下文里查
- 当前发送仍然要求对方至少先来过一条消息

### 发送媒体

```go
err := eng.SendMedia(ctx, accountID, toUserID, "image", "/abs/path/demo.jpg", "图片说明", "")
```

`mediaType` 当前支持：

- `image`
- `video`
- `file`

说明：

- `text` 不为空时，会先发文本，再发媒体
- 音频内容发送当前不可用

## HTTP API

当前可用接口：

- `GET /health/live`
- `GET /health/ready`
- `GET /api/version`
- `POST /api/accounts/login/start`
- `GET /api/accounts/login/status`
- `GET /api/accounts/login/qr`
- `GET /api/accounts`
- `GET /api/events`
- `GET /api/settings`
- `POST /api/settings`
- `POST /api/messages/send-text`
- `POST /api/messages/send-media`
- `POST /api/bot/getconfig` (获取 Bot 配置，包括 typing_ticket)
- `POST /api/bot/sendtyping` (发送"正在输入"状态)
- `POST /api/bot/notifystart` (通知 Bot 启动)
- `POST /api/bot/notifystop` (通知 Bot 停止)
- `GET /api/wecom/accounts`
- `POST /api/wecom/accounts`
- `DELETE /api/wecom/accounts`
- `GET /api/wecom/events`
- `POST /api/wecom/messages/send-text`
- `GET /api/wecom/contacts/user` (查询用户信息)
- `GET /api/wecom/contacts/users` (查询部门成员列表)
- `GET /api/wecom/contacts/departments` (查询部门列表)
- `GET /api/wecom/contacts/groupchat` (查询群聊详情)
- `GET /api/wecom/callback` (企业微信回调验证)
- `POST /api/wecom/callback` (企业微信消息接收)
- `GET /api/support/kb` (知识库列表)
- `GET /api/support/kb/search?q=关键词` (知识库搜索)
- `POST /api/support/kb` (添加知识库条目)
- `GET /api/support/kb/{id}` (知识库条目详情)
- `PUT /api/support/kb/{id}` (更新知识库条目)
- `DELETE /api/support/kb/{id}` (删除知识库条目)
- `GET /api/support/tickets` (工单列表)
- `POST /api/support/tickets` (创建工单)
- `GET /api/support/tickets/{id}` (工单详情)
- `PUT /api/support/tickets/{id}` (更新工单)
- `GET /api/support/orders` (订单列表)
- `POST /api/support/orders` (创建订单)
- `GET /api/support/orders/{id}` (订单详情)
- `POST /api/support/orders/{id}/refund` (订单退款)
- `GET /api/support/profiles` (客服规范列表)
- `POST /api/support/profiles` (创建客服规范)
- `GET /api/support/profiles/{id}` (客服规范详情)
- `PUT /api/support/profiles/{id}` (更新客服规范)
- `DELETE /api/support/profiles/{id}` (删除客服规范)
- `POST /api/support/profiles/{id}/default` (设为默认客服规范)
- `GET /api/agent/status` (Agent 状态)
- `GET /api/agent/conversations` (会话列表)
- `GET /api/agent/conversations/{id}` (会话详情与消息历史)
- `DELETE /api/agent/conversations/{id}` (删除会话)
- `POST /api/agent/chat` (HTTP 直接对话)

### 查询账号

```bash
curl -s http://127.0.0.1:17890/api/accounts
```

### 查询版本

```bash
curl -s http://127.0.0.1:17890/api/version
```

### 查询事件

```bash
curl -s "http://127.0.0.1:17890/api/events?after_id=0&limit=100"
```

返回的事件里会包含：

- `direction`
- `event_type`
- `from_user_id`
- `to_user_id`
- `group_id`（群消息时为群 ID）
- `body_text`
- `media_path`
- `media_file_name`
- `media_mime_type`

### 发送文本

```bash
curl -s -X POST http://127.0.0.1:17890/api/messages/send-text \
  -H 'Content-Type: application/json' \
  -d '{
    "account_id": "xxx@im.bot",
    "to_user_id": "yyy@im.wechat",
    "text": "你好"
  }'
```

### 发送媒体

```bash
curl -s -X POST http://127.0.0.1:17890/api/messages/send-media \
  -H 'Content-Type: application/json' \
  -d '{
    "account_id": "xxx@im.bot",
    "to_user_id": "yyy@im.wechat",
    "type": "image",
    "file_path": "/absolute/path/to/demo.jpg",
    "text": "图片说明"
  }'
```

说明：

- `type` 可传 `image`、`video`、`file`
- `text` 可选
- 当前音频内容发送不可用

### 获取 Bot 配置（typing_ticket）

```bash
curl -s -X POST http://127.0.0.1:17890/api/bot/getconfig \
  -H 'Content-Type: application/json' \
  -d '{
    "account_id": "xxx@im.bot",
    "ilink_user_id": "yyy@im.wechat",
    "context_token": "optional-context-token"
  }'
```

返回：

```json
{"typing_ticket": "base64-encoded-ticket"}
```

### 发送"正在输入"状态

```bash
curl -s -X POST http://127.0.0.1:17890/api/bot/sendtyping \
  -H 'Content-Type: application/json' \
  -d '{
    "account_id": "xxx@im.bot",
    "ilink_user_id": "yyy@im.wechat",
    "typing_ticket": "从 getconfig 获取的 ticket",
    "status": 1
  }'
```

说明：

- `status`：1 = 正在输入（默认），2 = 取消输入
- 用户端会看到"对方正在输入..."提示

### 通知 Bot 启动 / 停止

```bash
# 通知启动
curl -s -X POST http://127.0.0.1:17890/api/bot/notifystart \
  -H 'Content-Type: application/json' \
  -d '{"account_id": "xxx@im.bot"}'

# 通知停止
curl -s -X POST http://127.0.0.1:17890/api/bot/notifystop \
  -H 'Content-Type: application/json' \
  -d '{"account_id": "xxx@im.bot"}'
```

### 消息中的新字段

入站消息（`WeixinMessage`）新增以下字段：

| 字段 | 说明 |
|------|------|
| `group_id` | 群 ID（群消息时独立于 `from_user_id`） |
| `session_id` | 会话 ID |
| `run_id` | 运行 ID |
| `update_time_ms` | 消息更新时间 |
| `delete_time_ms` | 消息删除时间 |

消息项（`MessageItem`）新增：

| 字段 | 说明 |
|------|------|
| `ref_msg` | 引用消息（包含被引用的消息内容和摘要） |
| `msg_id` | 消息项 ID |
| `tool_call_start_item` | Tool Call 开始（type=11） |
| `tool_call_result_item` | Tool Call 结果（type=12） |

## 媒体文件保存

入站媒体默认保存到：

```text
<state-dir>/media/
```

事件记录中会保存：

- `media_path`
- `media_file_name`
- `media_mime_type`

## 配置

支持环境变量：

- `WCFLINK_LISTEN_ADDR`
- `WCFLINK_STATE_DIR`
- `WCFLINK_DB_PATH`
- `WCFLINK_MEDIA_DIR`
- `WCFLINK_BASE_URL`
- `WCFLINK_CDN_BASE_URL`
- `WCFLINK_CHANNEL_VERSION`
- `WCFLINK_POLL_TIMEOUT`
- `WCFLINK_LOG_LEVEL`

默认配置：

- 数据目录：`./bin/data/`
- 数据库：`./bin/data/wcfLink.db`
- 媒体目录：`<state-dir>/media/`

## 企业微信（WeCom）接入

### 前置准备

1. 登录[企业微信管理后台](https://work.weixin.qq.com/wework_admin/frame)
2. 创建自建应用，记录 `AgentId`
3. 在应用详情页获取 `Secret`
4. 在「企业信息」页获取 `CorpId`
5. 在应用的「接收消息」设置中配置回调 URL（指向 `http://<你的服务>/api/wecom/callback`），记录 `Token` 和 `EncodingAESKey`

### 配置

通过环境变量配置企业微信：

```bash
export WCFLINK_WECOM_CORP_ID="ww1234567890"          # 企业 ID
export WCFLINK_WECOM_CORP_SECRET="your-secret"        # 应用 Secret
export WCFLINK_WECOM_AGENT_ID="1000002"               # 应用 AgentId
export WCFLINK_WECOM_CALLBACK_TOKEN="your-token"      # 回调 Token
export WCFLINK_WECOM_CALLBACK_AES_KEY="your-aes-key"  # 回调 EncodingAESKey
export WCFLINK_WECOM_AUTO_REPLY="true"                # 启用自动回复
export WCFLINK_WECOM_WEBHOOK_URL="http://your-agent/webhook"  # Agent webhook（可选）
```

### 自动回复机制

当 `WCFLINK_WECOM_AUTO_REPLY=true` 时，入站消息会触发自动回复：

1. **webhook 模式**：如果配置了 `WCFLINK_WECOM_WEBHOOK_URL`，会将入站消息 POST 到该 URL，并将响应中的 `reply` 或 `text` 字段内容回复给用户
2. **echo 模式**：如果没有配置 webhook，会自动回复一条确认消息

Webhook 请求 payload 格式：

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

Webhook 响应格式（JSON）：

```json
{"reply": "你好，这是自动回复"}
```

也可以直接返回纯文本作为回复内容。

### 通过 HTTP API 管理企业微信

添加企业微信账号：

```bash
curl -s -X POST http://127.0.0.1:17890/api/wecom/accounts \
  -H 'Content-Type: application/json' \
  -d '{
    "corp_id": "ww1234567890",
    "corp_secret": "your-secret",
    "agent_id": 1000002,
    "callback_token": "your-token",
    "callback_aes_key": "your-aes-key",
    "auto_reply": true,
    "webhook_url": "http://your-agent/webhook"
  }'
```

查询企业微信账号：

```bash
curl -s http://127.0.0.1:17890/api/wecom/accounts
```

查询企业微信事件：

```bash
curl -s "http://127.0.0.1:17890/api/wecom/events?after_id=0&limit=100"
```

主动发送文本消息：

```bash
curl -s -X POST http://127.0.0.1:17890/api/wecom/messages/send-text \
  -H 'Content-Type: application/json' \
  -d '{
    "corp_id": "ww1234567890",
    "corp_secret": "your-secret",
    "agent_id": 1000002,
    "to_user": "UserName",
    "text": "你好"
  }'
```

查询用户信息：

```bash
curl -s "http://127.0.0.1:17890/api/wecom/contacts/user?corp_id=ww1234567890&corp_secret=your-secret&user_id=zhangsan"
```

查询部门成员列表（`department_id` 默认为 1，即根部门）：

```bash
curl -s "http://127.0.0.1:17890/api/wecom/contacts/users?corp_id=ww1234567890&corp_secret=your-secret&department_id=1"
```

查询部门列表：

```bash
curl -s "http://127.0.0.1:17890/api/wecom/contacts/departments?corp_id=ww1234567890&corp_secret=your-secret"
```

查询群聊详情（群成员列表）：

```bash
curl -s "http://127.0.0.1:17890/api/wecom/contacts/groupchat?corp_id=ww1234567890&corp_secret=your-secret&chat_id=CHATID"
```

### 作为 Go 库使用企业微信

```go
// 发送文本（支持 @：在文本中使用 <@UserID> 格式）
err := eng.WeComSendText(ctx, corpID, corpSecret, agentID, "UserName", "你好 <@zhangsan>")

// 查询企业微信账号
accounts, err := eng.WeComListAccounts(ctx)

// 查询企业微信事件
events, err := eng.WeComListEvents(ctx, 0, 100)

// 查询用户信息
user, err := eng.WeComGetUser(ctx, corpID, corpSecret, "zhangsan")

// 查询部门成员列表
users, err := eng.WeComListDepartmentUsers(ctx, corpID, corpSecret, 1)

// 查询部门列表
departments, err := eng.WeComListDepartments(ctx, corpID, corpSecret)

// 查询群聊详情
chat, err := eng.WeComGetGroupChat(ctx, corpID, corpSecret, "CHATID")
```

### 企业微信环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WCFLINK_WECOM_CORP_ID` | 企业 ID | 空 |
| `WCFLINK_WECOM_CORP_SECRET` | 应用 Secret | 空 |
| `WCFLINK_WECOM_AGENT_ID` | 应用 AgentId | 0 |
| `WCFLINK_WECOM_CALLBACK_TOKEN` | 回调 Token | 空 |
| `WCFLINK_WECOM_CALLBACK_AES_KEY` | 回调 EncodingAESKey | 空 |
| `WCFLINK_WECOM_API_BASE_URL` | 企业微信 API 地址 | `https://qyapi.weixin.qq.com` |
| `WCFLINK_WECOM_AUTO_REPLY` | 启用自动回复 | `false` |
| `WCFLINK_WECOM_WEBHOOK_URL` | 自动回复 webhook 地址 | 空 |

## AI Agent 功能

### 概述

wcfLink 内置 AI Agent 引擎，可以将微信 Bot 升级为智能助手。Agent 通过 LLM（大语言模型）进行多轮对话，支持工具调用（搜索、网页抓取），可以帮助用户进行市场分析、撰写 PRD、设计原型等。

### 工作流程

```
微信用户发消息
  → wcfLink 接收（iLink / WeCom）
  → Agent 引擎处理：
      1. 加载会话历史
      2. 调用 LLM（DeepSeek/OpenAI/...）
      3. LLM 决策：追问用户 / 调用工具 / 返回最终回复
      4. 工具调用结果注入上下文，继续循环
      5. 生成最终回复
  → wcfLink 发送回复（自动分段）
```

### 配置

```bash
# 启用 Agent
export WCFLINK_AGENT_ENABLED="true"

# LLM 配置（支持 OpenAI Compatible API）
export WCFLINK_LLM_BASE_URL="https://api.deepseek.com"  # 或 OpenAI / 阿里云 / 火山引擎
export WCFLINK_LLM_API_KEY="sk-xxx"
export WCFLINK_LLM_MODEL="deepseek-chat"
export WCFLINK_LLM_TEMPERATURE="0.7"
export WCFLINK_LLM_MAX_TOKENS="4096"

# Agent 可选配置
export WCFLINK_AGENT_DEFAULT_MODE="icemark"          # 默认模式
export WCFLINK_AGENT_MAX_ITERATIONS="10"             # 工具调用最大迭代次数
export WCFLINK_AGENT_SESSION_TTL="168h"              # 会话过期时间（默认 7 天）
export WCFLINK_FETCH_MAX_CONTENT_LENGTH="8000"       # 网页抓取最大字符数
```

### 模式系统

Agent 支持五种专业模式，用户通过命令切换：

| 命令 | 模式 | 说明 |
|------|------|------|
| `/icemark` | 通用助手 | 默认模式，通用规划、分析、执行 |
| `/market` | 市场分析 | SWOT、PESTEL、波特五力等分析框架 |
| `/prd` | PRD 文档 | 用户故事、JTBD、三问法需求挖掘 |
| `/prototype` | 原型设计 | 快速生成 HTML 交互原型 |
| `/support` | 客服助手 | 专业客服模式，遵循 SOP 规范，集成知识库/工单/订单 |
| `/support-setup` | 配置向导 | 通过多轮对话生成自定义客服规范 |
| `/support-profiles` | - | 查看所有客服规范配置 |
| `/support-use <名称>` | - | 切换默认客服规范 |
| `/reset` | - | 清空当前会话历史 |
| `/mode` | - | 查看当前模式 |
| `/help` | - | 显示帮助信息 |

### 内置工具

| 工具 | 说明 | 可用模式 |
|------|------|----------|
| `web_search` | 多引擎搜索（DuckDuckGo → Bing 降级），支持通用搜索和平台搜索 | 所有模式 |
| `url_content_fetch` | 获取指定 URL 的网页内容，转换为纯文本格式 | 所有模式 |
| `kb_search` | 搜索知识库（FAQ/产品文档），按关键词匹配 | 客服模式 |
| `ticket_create` | 创建客服工单，记录客户问题 | 客服模式 |
| `ticket_query` | 查询工单列表或指定工单详情 | 客服模式 |
| `ticket_update` | 更新工单状态、优先级、备注 | 客服模式 |
| `order_query` | 查询订单信息 | 客服模式 |
| `order_create` | 创建新订单记录 | 客服模式 |
| `order_refund` | 处理订单退款 | 客服模式 |

### 架构

```
微信用户消息
  │
  ├── iLink 通道 (个人微信)
  │   └── HandleInboundMessage → 提取文本 → Agent.HandleMessage(ctx, SessionKey, text)
  │
  └── WeCom 通道 (企业微信)
      └── HandleInbound → 提取文本 → Agent.HandleMessage(ctx, SessionKey, text)
                                            │
                                            ▼
                                    ┌─────────────────┐
                                    │   Agent 主循环    │
                                    │                   │
                                    │  1. 加载会话历史    │
                                    │  2. 拼接系统提示词  │
                                    │  3. 调用 LLM       │
                                    │  4. 判断 finish:    │
                                    │     tool_calls →   │
                                    │       执行工具 →    │
                                    │       注入结果 →    │
                                    │       继续循环      │
                                    │     stop →         │
                                    │       发送回复      │
                                    └─────────────────┘
                                            │
                                            ▼
                                    MultiChannelSender
                                    ├── iLink: SendText(accountID, toUserID, text, contextToken)
                                    └── WeCom: SendText(corpID, corpSecret, agentID, toUser, text)
```

Agent 启用后，入站文本消息自动路由到 Agent 处理，不再转发到 webhook。Agent 未启用时完全不影响现有 webhook 流程。

### 客服助手模式（三层架构）

客服助手模式基于三层架构设计：

```
① Builder 层（/support-setup）
   通过多轮对话收集业务信息，动态生成客服规范配置
   ├── 收集：公司名、行业、问候语、退款政策、升级阈值、营业时间
   └── 输出：完整的客服规范 Profile
                │
② Behavior 层（SystemPrompt）
   定义客服服务规范（SOP）
   ├── 服务流程：倾听 → 共情 → 确认 → 解决 → 确认满意 → 跟进
   ├── LATTE 投诉模型：Listen → Acknowledge → Take action → Thank → Explain
   ├── 语气规范 + 共情话术 + 禁止话术
   └── 升级策略：法律/高额退款/安全/媒体 → 必须升级
                │
③ Runtime 层（Agent 主循环 + 工具）
   实际执行客服工作
   ├── 消息分类 → 知识库搜索 → 工具调用 → 生成回复
   ├── 工具：kb_search / ticket_create / ticket_query / ticket_update
   │         order_query / order_create / order_refund
   └── 升级输出："[需升级] 原因 + 客户信息 + 建议处理"
```

**方式 A（预配置默认）+ 方式 B（动态生成）：**

- `/support` — 使用默认客服规范进入客服模式
- `/support-setup` — 启动配置向导，通过多轮对话生成自定义规范
- `/support-profiles` — 查看所有已创建的客服规范
- `/support-use <名称>` — 切换默认客服规范

### 数据库表

Agent 使用与 wcfLink 相同的 SQLite 数据库，新增以下表：

| 表 | 说明 |
|---|------|
| `conversations` | 会话元数据（channel_type + user_id + group_id 唯一索引） |
| `conversation_messages` | 消息历史（role/content/tool_calls/tool_call_id） |
| `tool_call_logs` | 工具调用日志（名称、参数、结果、耗时、错误） |
| `kb_articles` | 知识库条目（FAQ/产品文档，支持分类、标签、优先级） |
| `tickets` | 工单（主题、描述、状态、优先级、分类、处理人） |
| `orders` | 订单（产品、金额、状态、退款信息） |
| `support_profiles` | 客服规范配置（公司信息、退款政策、升级阈值等） |

表在 Agent 首次启动时自动创建。

### Agent HTTP API

查询 Agent 状态：

```bash
curl -s http://127.0.0.1:17890/api/agent/status
```

通过 HTTP API 直接与 Agent 对话：

```bash
curl -s -X POST http://127.0.0.1:17890/api/agent/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id": "test-user-1",
    "message": "帮我分析一下 AI 编程助手的市场趋势"
  }'
```

查询会话列表：

```bash
curl -s http://127.0.0.1:17890/api/agent/conversations
```

查询某个会话的详情和消息历史：

```bash
curl -s http://127.0.0.1:17890/api/agent/conversations/{conversation_id}
```

删除会话：

```bash
curl -s -X DELETE http://127.0.0.1:17890/api/agent/conversations/{conversation_id}
```

### Agent 用户交互示例

**通用模式：**

```
用户: 帮我分析一下 AI 编程助手的市场趋势
Bot:  好的，我来搜索一下最新的市场信息...
      [调用 web_search: "AI 编程助手 市场趋势 2026"]
      [调用 url_content_fetch: 获取详细报告内容]
Bot:  ## AI 编程助手市场分析
      ### 市场规模 ...
      ### 主要玩家 ...
      需要我深入分析某个方面吗？
```

**客服模式：**

```
用户: /support
Bot:  已切换到客服助手模式。

用户: 我买的东西一直没发货，订单号是 xxx
Bot:  [调用 order_query: 查询订单]
      我理解您着急等快递的心情。我查到您的订单目前状态是"已付款"，
      确实还未发货。我现在就为您创建一个跟踪工单...
      [调用 ticket_create: 创建催发货工单]
      工单已创建（ID: abc123），我们会尽快跟进处理，
      预计 24 小时内给您回复。还有其他问题吗？
```

**配置向导：**

```
用户: /support-setup
Bot:  已进入客服规范配置向导。请告诉我你想配置什么类型的客服规范？

用户: 电商客服
Bot:  好的！请问你的公司/品牌名称是什么？

用户: 优品商城
Bot:  收到。退款政策是怎样的？比如多少天内可退、退款金额上限等？

用户: 7天无理由退货，金额不限
Bot:  明白了。我来确认一下配置...
      客服规范「电商客服」已创建成功！使用 /support 切换到客服模式即可生效。
```

**切换客服规范：**

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
# 添加知识库条目
curl -s -X POST http://127.0.0.1:17890/api/support/kb \
  -H 'Content-Type: application/json' \
  -d '{
    "category": "退款",
    "question": "如何申请退款？",
    "answer": "7天内无理由退款，请提供订单号。",
    "tags": "退款,售后",
    "priority": 10
  }'

# 搜索知识库
curl -s "http://127.0.0.1:17890/api/support/kb/search?q=退款"

# 列出知识库
curl -s "http://127.0.0.1:17890/api/support/kb?category=退款"
```

**工单管理：**

```bash
# 创建工单
curl -s -X POST http://127.0.0.1:17890/api/support/tickets \
  -H 'Content-Type: application/json' \
  -d '{
    "subject": "订单未发货",
    "description": "客户反映订单超过3天未发货",
    "customer_id": "user123",
    "priority": "high",
    "category": "物流"
  }'

# 查询工单（支持过滤）
curl -s "http://127.0.0.1:17890/api/support/tickets?status=open&priority=high"

# 更新工单
curl -s -X PUT http://127.0.0.1:17890/api/support/tickets/{id} \
  -H 'Content-Type: application/json' \
  -d '{"status": "resolved", "notes": "已联系物流，预计明天到"}'
```

**订单管理：**

```bash
# 创建订单
curl -s -X POST http://127.0.0.1:17890/api/support/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "customer_id": "user123",
    "product": "年度会员",
    "amount": 299.00,
    "payment_method": "微信支付"
  }'

# 查询订单
curl -s "http://127.0.0.1:17890/api/support/orders?customer_id=user123"

# 订单退款
curl -s -X POST http://127.0.0.1:17890/api/support/orders/{id}/refund \
  -H 'Content-Type: application/json' \
  -d '{"amount": 299.00, "reason": "7天无理由退款"}'
```

**客服规范管理：**

```bash
# 创建客服规范
curl -s -X POST http://127.0.0.1:17890/api/support/profiles \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "电商客服",
    "company_name": "优品商城",
    "industry": "电商",
    "greeting": "您好！欢迎来到优品商城，请问有什么可以帮您？",
    "escalation_threshold": 500,
    "refund_policy": "7天无理由退款",
    "business_hours": "周一至周五 9:00-18:00",
    "is_default": true
  }'

# 列出所有规范
curl -s http://127.0.0.1:17890/api/support/profiles

# 设为默认规范
curl -s -X POST http://127.0.0.1:17890/api/support/profiles/{id}/default
```

### Agent 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `WCFLINK_AGENT_ENABLED` | 启用 Agent | `false` |
| `WCFLINK_AGENT_DEFAULT_MODE` | 默认模式 | `icemark` |
| `WCFLINK_AGENT_MAX_ITERATIONS` | 最大工具调用迭代次数 | `10` |
| `WCFLINK_AGENT_SESSION_TTL` | 会话过期时间 | `168h`（7 天） |
| `WCFLINK_LLM_BASE_URL` | LLM API 地址 | `https://api.deepseek.com` |
| `WCFLINK_LLM_API_KEY` | LLM API Key | 空（必填） |
| `WCFLINK_LLM_MODEL` | 模型名称 | `deepseek-chat` |
| `WCFLINK_LLM_TEMPERATURE` | 生成温度 | `0.7` |
| `WCFLINK_LLM_MAX_TOKENS` | 最大输出 token 数 | `4096` |
| `WCFLINK_FETCH_MAX_CONTENT_LENGTH` | 网页抓取最大字符数 | `8000` |
