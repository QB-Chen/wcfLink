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
- 滑动窗口上下文管理
- 内置工具：网页搜索（DuckDuckGo/Bing 多引擎降级）、网页内容抓取
- 四种专业模式：通用助手（Icemark）、市场分析（Market）、PRD 文档、原型设计（Prototype）
- 命令系统：`/icemark`、`/market`、`/prd`、`/prototype`、`/reset`、`/mode`、`/help`
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

Agent 支持四种专业模式，用户通过命令切换：

| 命令 | 模式 | 说明 |
|------|------|------|
| `/icemark` | 通用助手 | 默认模式，通用规划、分析、执行 |
| `/market` | 市场分析 | SWOT、PESTEL、波特五力等分析框架 |
| `/prd` | PRD 文档 | 用户故事、JTBD、三问法需求挖掘 |
| `/prototype` | 原型设计 | 快速生成 HTML 交互原型 |
| `/reset` | - | 清空当前会话历史 |
| `/mode` | - | 查看当前模式 |
| `/help` | - | 显示帮助信息 |

### 内置工具

| 工具 | 说明 |
|------|------|
| `web_search` | 多引擎搜索（DuckDuckGo → Bing 降级），支持通用搜索和平台搜索（小红书/知乎/微博） |
| `url_content_fetch` | 获取指定 URL 的网页内容，转换为纯文本格式 |

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
