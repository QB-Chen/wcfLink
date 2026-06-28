# wcfLink AI Agent 功能需求文档

## 1. 背景与目标

### 1.1 背景

wcfLink 当前是一个微信消息中继服务，支持 iLink 个人微信通道和企业微信（WeCom）通道。消息接收后通过 webhook 转发到外部服务处理，自身不具备智能处理能力。

用户希望 wcfLink 内置 AI Agent 能力，使微信 Bot 能够：
- 与用户进行多轮对话，反复确认需求
- 在需求明确后调用工具（搜索、网页抓取等）完成任务
- 支持不同工作模式（通用助手、市场分析、PRD 编写、原型生成）

参考项目：[Icemark](https://github.com/kakuka/icemark) — 一个基于 VSCode 的产品经理 AI Agent。

### 1.2 目标

将 wcfLink 从"消息中继"升级为"微信 AI Agent"，用户通过微信（个人/企业）与 Agent 对话，Agent 能理解需求、主动提问确认、调用工具执行任务、返回结构化结果。

### 1.3 核心价值

- **零部署门槛**：不需要额外的 AI 服务或 webhook，wcfLink 自身即可完成智能对话
- **多轮交互**：Agent 不是一问一答，而是通过反复确认来深入理解用户需求
- **工具增强**：Agent 可以搜索互联网、抓取网页内容、分析数据，不只是 LLM 生成文本
- **双通道**：个人微信和企业微信统一接入同一个 Agent

---

## 2. 用户场景

### 2.1 场景一：通用助手（Icemark 模式）

```
用户: 帮我了解一下低代码平台的发展趋势
Agent: 好的，我有几个问题想确认一下：
       1. 你关注的是国内还是全球市场？
       2. 有没有特别关注的低代码产品？
       3. 需要重点关注哪些方面？（市场规模/技术趋势/竞争格局/用户画像）
用户: 国内市场，重点看市场规模和竞争格局
Agent: 收到，我开始搜索和整理相关资料...
       [调用 web_search: "2024 2025 中国低代码平台市场规模"]
       [调用 web_search: "低代码平台 竞争格局 市场份额"]
       [调用 url_fetch: 抓取相关报告页面]
Agent: ## 中国低代码平台市场分析
       ### 市场规模
       - 2024年市场规模约为...
       ### 竞争格局
       - 头部玩家：...
       
       需要我深入分析某个方面吗？
用户: 再看看钉钉宜搭和微搭的对比
Agent: [继续调用工具补充分析]
```

### 2.2 场景二：市场分析（Market 模式）

```
用户: /market
Agent: 已切换到市场分析模式。请告诉我：
       - 要分析的目标市场或行业是什么？
       - 分析的主要目的是什么？
       - 有特别关注的方面吗？
用户: 分析国内 AI 编程助手市场，目的是评估进入机会
Agent: 明白。我还需要确认：
       - 分析的地理范围：仅中国大陆，还是包含港澳台？
       - 时间范围：近几年的历史数据 + 未来多少年的预测？
       - 是否需要特别关注某些竞品？
用户: 大陆市场，近2年历史+未来3年预测，重点看通义灵码和GitHub Copilot
Agent: 收到，开始执行分析...
       [多轮工具调用：搜索、抓取、分析]
Agent: ## AI 编程助手市场分析报告
       ### 1. 市场概况 ...
       ### 2. 竞争格局 ...
       ### 3. SWOT 分析 ...
       ### 4. 进入机会评估 ...
       
       这份报告是否需要补充或调整？
```

### 2.3 场景三：PRD 编写（PRD 模式）

```
用户: /prd
Agent: 已切换到 PRD 模式。我会帮你创建产品需求文档。
       首先，请告诉我：
       - 目标用户是谁？
       - 用户想要实现什么目标？
       - 当前用户如何完成相关任务？存在哪些痛点？
用户: 目标用户是小型电商卖家，他们需要一个自动回复客户咨询的工具
Agent: 谢谢。我需要进一步了解：
       - 客户咨询主要来自哪些渠道？（微信/淘宝/抖音/...）
       - 当前卖家是怎么处理客户咨询的？
       - 最大的痛点是什么？（回复速度慢/人工成本高/回复质量不一致/...）
       - 对自动回复的准确率有什么期望？
用户: 主要是微信和淘宝，目前人工回复，痛点是夜间无人值守和高峰期回复慢
Agent: 收到。让我搜索一下同类产品的情况...
       [调用搜索工具了解竞品]
Agent: 基于以上信息，我提议的 PRD 结构如下：
       1. 背景与目标
       2. 用户故事与 JTBD 分析
       3. 四种力量分析
       4. 当前流程分析
       5. 解决方案描述
       6. 功能需求列表
       7. 成功指标
       
       可以开始生成吗？需要调整结构吗？
用户: 可以，开始吧
Agent: [生成完整 PRD 文档，分段发送]
```

### 2.4 场景四：原型生成（Prototype 模式）

```
用户: /prototype
Agent: 已切换到原型模式。请告诉我：
       - 原型的目标平台？（Web / 移动端 / 桌面端）
       - 需要包含哪些页面或功能？
       - 有没有 UI 风格偏好？
用户: 移动端，做一个简单的待办事项 App，包含首页列表和新增页面
Agent: 明白。确认一下细节：
       - 首页列表需要支持分类或标签吗？
       - 待办项需要设置截止日期和优先级吗？
       - 需要完成/删除功能吗？
用户: 需要优先级和截止日期，支持完成和删除
Agent: 好的，我会生成以下页面：
       1. index.html — 待办列表首页（含筛选和排序）
       2. add.html — 新增待办页面
       3. detail.html — 待办详情页面
       
       确认后开始生成？
用户: 确认
Agent: [生成 HTML 原型文件]
Agent: 原型已生成。文件列表：
       - index.html（待办列表）
       - add.html（新增待办）
       - detail.html（待办详情）
       [可通过文件消息发送 HTML 文件]
```

---

## 3. 系统架构

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                     wcfLink 进程                         │
│                                                         │
│  ┌──────────┐    ┌──────────┐    ┌──────────────────┐  │
│  │  iLink   │    │  WeCom   │    │   HTTP API       │  │
│  │  通道    │    │  通道    │    │   /api/agent/... │  │
│  └────┬─────┘    └────┬─────┘    └────────┬─────────┘  │
│       │               │                    │            │
│       └───────┬───────┘                    │            │
│               ▼                            │            │
│  ┌──────────────────────────────┐          │            │
│  │     消息路由层 (Router)       │◄─────────┘            │
│  │  判断：走 Agent / 走 Webhook │                       │
│  └──────────┬───────────────────┘                       │
│             │                                           │
│             ▼                                           │
│  ┌──────────────────────────────────────────────┐      │
│  │              Agent 引擎                       │      │
│  │                                              │      │
│  │  ┌────────────────┐  ┌────────────────────┐  │      │
│  │  │ 会话管理器      │  │ 模式管理器          │  │      │
│  │  │ (Conversation) │  │ (Mode Manager)     │  │      │
│  │  └────────┬───────┘  └────────┬───────────┘  │      │
│  │           │                   │              │      │
│  │           ▼                   ▼              │      │
│  │  ┌──────────────────────────────────────┐    │      │
│  │  │         Agent 主循环                  │    │      │
│  │  │                                      │    │      │
│  │  │  用户消息 → LLM 推理 → 决策：        │    │      │
│  │  │    ├─ 问用户 → 发微信消息，等回复     │    │      │
│  │  │    ├─ 调工具 → 执行，结果回 LLM      │    │      │
│  │  │    └─ 最终回复 → 发微信消息           │    │      │
│  │  └──────────────────────────────────────┘    │      │
│  │           │                                  │      │
│  │           ▼                                  │      │
│  │  ┌──────────────────────────────────────┐    │      │
│  │  │         工具系统 (Tools)              │    │      │
│  │  │                                      │    │      │
│  │  │  ┌─────────┐ ┌──────────┐ ┌───────┐ │    │      │
│  │  │  │web_search│ │url_fetch │ │social │ │    │      │
│  │  │  │(多引擎)  │ │(网页抓取)│ │search │ │    │      │
│  │  │  └─────────┘ └──────────┘ └───────┘ │    │      │
│  │  └──────────────────────────────────────┘    │      │
│  │           │                                  │      │
│  │           ▼                                  │      │
│  │  ┌──────────────────────────────────────┐    │      │
│  │  │         LLM 客户端                    │    │      │
│  │  │  OpenAI Compatible API               │    │      │
│  │  │  (DeepSeek / OpenAI / 通义 / ...)    │    │      │
│  │  └──────────────────────────────────────┘    │      │
│  └──────────────────────────────────────────────┘      │
│                                                         │
│  ┌──────────────────────────────────────────────┐      │
│  │              SQLite 存储                      │      │
│  │  conversations | messages | tool_calls       │      │
│  └──────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────┘
```

### 3.2 消息路由逻辑

当 Agent 功能开启时（`WCFLINK_AGENT_ENABLED=true`），入站消息的处理流程：

```
入站消息
  │
  ├─ Agent 未启用 → 走原有 webhook 逻辑（不变）
  │
  └─ Agent 已启用
      │
      ├─ 是命令消息（以 / 开头）
      │   ├─ /market  → 切换到市场分析模式
      │   ├─ /prd     → 切换到 PRD 模式
      │   ├─ /prototype → 切换到原型模式
      │   ├─ /icemark → 切换到通用助手模式
      │   ├─ /reset   → 清空当前会话历史
      │   └─ /help    → 显示帮助信息
      │
      └─ 是普通消息 → 进入 Agent 主循环
```

---

## 4. 核心模块设计

### 4.1 LLM 客户端 (`internal/llm/`)

#### 4.1.1 接口定义

```go
// internal/llm/client.go

type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

type Message struct {
    Role       Role        `json:"role"`
    Content    string      `json:"content,omitempty"`
    ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
    ToolCallID string      `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
    ID       string       `json:"id"`
    Type     string       `json:"type"` // "function"
    Function FunctionCall `json:"function"`
}

type FunctionCall struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"` // JSON string
}

type ToolDefinition struct {
    Type     string              `json:"type"` // "function"
    Function FunctionDefinition  `json:"function"`
}

type FunctionDefinition struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Parameters  interface{} `json:"parameters"` // JSON Schema
}

type ChatRequest struct {
    Model       string           `json:"model"`
    Messages    []Message        `json:"messages"`
    Tools       []ToolDefinition `json:"tools,omitempty"`
    Temperature float64          `json:"temperature,omitempty"`
    MaxTokens   int              `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
    ID      string   `json:"id"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

type Choice struct {
    Index        int     `json:"index"`
    Message      Message `json:"message"`
    FinishReason string  `json:"finish_reason"` // "stop" | "tool_calls"
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

type Client interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
```

#### 4.1.2 实现

采用 OpenAI Compatible API 标准，兼容以下提供商：

| 提供商 | Base URL | 说明 |
|--------|----------|------|
| DeepSeek | `https://api.deepseek.com` | 推荐，性价比高 |
| OpenAI | `https://api.openai.com` | GPT-4o 等 |
| 阿里云通义 | `https://dashscope.aliyuncs.com/compatible-mode` | qwen 系列 |
| 火山引擎 | 用户配置 | 豆包等 |
| 自定义 | 用户配置 | 任何 OpenAI 兼容 API |

```go
type OpenAICompatibleClient struct {
    baseURL    string
    apiKey     string
    model      string
    httpClient *http.Client
}

func NewClient(baseURL, apiKey, model string) Client {
    return &OpenAICompatibleClient{
        baseURL:    strings.TrimRight(baseURL, "/"),
        apiKey:     apiKey,
        model:      model,
        httpClient: &http.Client{Timeout: 120 * time.Second},
    }
}
```

#### 4.1.3 配置

```bash
WCFLINK_LLM_BASE_URL="https://api.deepseek.com"    # API 地址
WCFLINK_LLM_API_KEY="sk-xxx"                        # API 密钥
WCFLINK_LLM_MODEL="deepseek-chat"                   # 模型名称
WCFLINK_LLM_TEMPERATURE="0.7"                       # 温度（可选，默认 0.7）
WCFLINK_LLM_MAX_TOKENS="4096"                       # 最大输出 token（可选，默认 4096）
```

### 4.2 会话管理器 (`internal/agent/conversation.go`)

#### 4.2.1 数据模型

每个用户（或群 + 用户组合）维护独立的会话上下文。

```go
type Conversation struct {
    ID            string    // 会话 ID（UUID）
    ChannelType   string    // "ilink" | "wecom"
    UserID        string    // 用户标识（iLink: from_user_id, WeCom: FromUserName）
    GroupID       string    // 群 ID（私聊为空）
    Mode          string    // 当前模式 slug
    Messages      []Message // 对话历史（role/content）
    CreatedAt     time.Time
    UpdatedAt     time.Time
    TokenCount    int       // 已使用 token 数估算
}
```

#### 4.2.2 会话键（Session Key）

```
会话键 = channel_type + ":" + user_id + ":" + group_id
```

- 私聊：`ilink:wxid_xxx:` — 一个用户一个会话
- 群聊：`ilink:wxid_xxx:group123@chatroom` — 同一用户在不同群有不同会话
- 企业微信：`wecom:zhangsan:` — 同理

#### 4.2.3 上下文窗口管理

LLM 有 token 限制，需要管理上下文窗口：

```go
const (
    MaxContextTokens  = 32000  // 上下文窗口上限（可配置）
    ReservedTokens    = 4096   // 为输出保留的 token 数
    SystemPromptSlot  = 4000   // 系统提示词预留
)
```

策略：

1. **固定部分**：系统提示词（mode prompt）始终在最前面
2. **滑动窗口**：对话历史从最新往前保留，超过 `MaxContextTokens - ReservedTokens - SystemPromptSlot` 时，截断最早的消息
3. **摘要压缩（可选，第二阶段）**：被截断的历史先由 LLM 生成摘要，摘要放在系统提示词后面

#### 4.2.4 数据库 Schema

```sql
CREATE TABLE IF NOT EXISTS conversations (
    id            TEXT PRIMARY KEY,
    channel_type  TEXT NOT NULL,            -- "ilink" | "wecom"
    user_id       TEXT NOT NULL,
    group_id      TEXT NOT NULL DEFAULT '', -- 群 ID，私聊为空
    mode          TEXT NOT NULL DEFAULT 'icemark',
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    token_count   INTEGER DEFAULT 0
);

CREATE UNIQUE INDEX idx_conv_session_key ON conversations(channel_type, user_id, group_id);

CREATE TABLE IF NOT EXISTS conversation_messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    role            TEXT NOT NULL,           -- "system" | "user" | "assistant" | "tool"
    content         TEXT,
    tool_calls      TEXT,                    -- JSON: [{id, type, function: {name, arguments}}]
    tool_call_id    TEXT,                    -- 工具调用结果的关联 ID
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_conv_msg_conv_id ON conversation_messages(conversation_id);
```

### 4.3 Agent 主循环 (`internal/agent/agent.go`)

#### 4.3.1 核心流程

```go
func (a *Agent) HandleMessage(ctx context.Context, session SessionKey, userMessage string) error {
    // 1. 获取或创建会话
    conv := a.conversationMgr.GetOrCreate(session)

    // 2. 追加用户消息到会话历史
    conv.AddMessage(Message{Role: RoleUser, Content: userMessage})

    // 3. 进入 Agent 循环
    for iteration := 0; iteration < MaxIterations; iteration++ {
        // 3.1 构建 LLM 请求
        req := a.buildChatRequest(conv)

        // 3.2 调用 LLM
        resp, err := a.llmClient.Chat(ctx, req)
        if err != nil {
            return a.sendError(ctx, session, err)
        }

        assistantMsg := resp.Choices[0].Message

        // 3.3 追加助手消息到会话历史
        conv.AddMessage(assistantMsg)

        // 3.4 判断 LLM 的决策
        switch resp.Choices[0].FinishReason {

        case "stop":
            // LLM 给出了最终回复（可能是回答，也可能是追问用户）
            // 发送消息给微信用户
            return a.sendReply(ctx, session, assistantMsg.Content)

        case "tool_calls":
            // LLM 要求调用工具
            for _, tc := range assistantMsg.ToolCalls {
                // 执行工具
                result, err := a.executeTool(ctx, tc)
                
                // 将工具结果追加到会话历史
                conv.AddMessage(Message{
                    Role:       RoleTool,
                    Content:    result,
                    ToolCallID: tc.ID,
                })
            }
            // 继续循环，让 LLM 根据工具结果决定下一步
            continue
        }
    }

    return a.sendReply(ctx, session, "抱歉，处理过程中超过了最大迭代次数，请重试或简化你的需求。")
}
```

#### 4.3.2 关键设计点

**多轮对话的实现**：

Agent 的"追问用户"不需要特殊机制。当 LLM 认为信息不足时，它会生成一条追问消息（`finish_reason="stop"`），wcfLink 将这条消息通过微信发给用户。用户回复后，iLink/WeCom 通道再次触发 `HandleMessage`，此时会话历史中已经包含了之前的所有上下文，LLM 自然能延续对话。

```
时间线：
  T1: 用户发 "帮我做个分析" → HandleMessage → LLM 返回追问 → 发微信 "分析什么？"
  T2: 用户发 "低代码市场"   → HandleMessage → LLM 返回追问 → 发微信 "哪些方面？"
  T3: 用户发 "市场规模"     → HandleMessage → LLM 调用 web_search → 调用 url_fetch → 返回结果
```

**会话隔离**：

每个 session key 对应独立的对话历史，不同用户、不同群聊之间完全隔离。

**并发安全**：

同一用户的消息串行处理（通过 per-session 锁），防止并发 LLM 调用导致会话历史混乱。

```go
type Agent struct {
    sessionLocks sync.Map // map[SessionKey]*sync.Mutex
}

func (a *Agent) getSessionLock(key SessionKey) *sync.Mutex {
    v, _ := a.sessionLocks.LoadOrStore(string(key), &sync.Mutex{})
    return v.(*sync.Mutex)
}
```

**最大迭代数**：

防止 Agent 无限循环调用工具。默认 `MaxIterations = 10`，即一轮用户消息最多触发 10 次 LLM 调用。

**"正在输入"状态**：

在 Agent 处理期间，调用 iLink 的 `sendtyping` 接口（status=1），让用户看到"对方正在输入..."的提示。处理完成后发送 status=2 取消。

### 4.4 工具系统 (`internal/agent/tools/`)

#### 4.4.1 工具注册接口

```go
type ToolExecutor interface {
    // Name 返回工具名称（与 LLM function calling 的 name 对应）
    Name() string
    
    // Definition 返回工具的 JSON Schema 定义（给 LLM 看的）
    Definition() ToolDefinition
    
    // Execute 执行工具调用，返回结果文本
    Execute(ctx context.Context, arguments string) (string, error)
}
```

#### 4.4.2 工具列表

**第一阶段工具：**

| 工具名 | 说明 | 参数 |
|--------|------|------|
| `web_search` | 互联网搜索 | `keyword_list` (逗号分隔), `page_limit` (1-10), `search_on` (general/xiaohongshu/zhihu/weibo) |
| `url_content_fetch` | 网页内容抓取，转为 Markdown | `url` |

**第二阶段工具：**

| 工具名 | 说明 | 参数 |
|--------|------|------|
| `social_search` | 社交平台专项搜索（小红书、知乎、微博） | `platform`, `keyword`, `limit` |
| `generate_report` | 生成结构化报告（Markdown） | `title`, `sections` |
| `generate_prototype` | 生成 HTML 原型 | `platform` (web/mobile/desktop), `pages` |

#### 4.4.3 web_search 工具实现

```go
// internal/agent/tools/web_search.go

type WebSearchTool struct {
    httpClient *http.Client
}

// LLM 看到的工具定义
func (t *WebSearchTool) Definition() ToolDefinition {
    return ToolDefinition{
        Type: "function",
        Function: FunctionDefinition{
            Name:        "web_search",
            Description: "搜索互联网获取最新信息。支持多个搜索引擎和平台。返回搜索结果列表（标题、URL、摘要）。",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "keyword_list": map[string]interface{}{
                        "type":        "string",
                        "description": "搜索关键词，多个关键词用逗号分隔",
                    },
                    "page_limit": map[string]interface{}{
                        "type":        "integer",
                        "description": "搜索页数（1-10），默认 1",
                        "default":     1,
                    },
                    "search_on": map[string]interface{}{
                        "type":        "string",
                        "description": "搜索平台：general（通用搜索引擎）、xiaohongshu（小红书）、zhihu（知乎）、weibo（微博）",
                        "enum":        []string{"general", "xiaohongshu", "zhihu", "weibo"},
                        "default":     "general",
                    },
                },
                "required": []string{"keyword_list"},
            },
        },
    }
}
```

搜索引擎实现方案（Go 语言原生实现，不依赖浏览器）：

| 引擎 | 方式 | 说明 |
|------|------|------|
| Bing | HTTP API | 调用 Bing Web Search API（需 API Key）或解析搜索页面 |
| 百度 | HTTP 请求 | 请求百度搜索页，解析 HTML 提取结果 |
| 搜狗 | HTTP 请求 | 同百度 |
| DuckDuckGo | HTTP API | DuckDuckGo Instant Answer API（免费） |
| 小红书 | HTTP 请求 | 小红书搜索页抓取 |
| 知乎 | HTTP 请求 | 知乎搜索 API |
| 微博 | HTTP 请求 | 微博搜索抓取 |

搜索引擎优先级：`DuckDuckGo → Bing → 百度 → 搜狗`（按可用性 fallback）。

#### 4.4.4 url_content_fetch 工具实现

```go
// internal/agent/tools/url_fetch.go

type URLFetchTool struct {
    httpClient *http.Client
}

func (t *URLFetchTool) Definition() ToolDefinition {
    return ToolDefinition{
        Type: "function",
        Function: FunctionDefinition{
            Name:        "url_content_fetch",
            Description: "获取指定 URL 的网页内容，转换为 Markdown 格式返回。适用于深入阅读搜索结果中的详细内容。",
            Parameters: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "url": map[string]interface{}{
                        "type":        "string",
                        "description": "要获取内容的网页 URL",
                    },
                },
                "required": []string{"url"},
            },
        },
    }
}
```

实现流程：

```
HTTP GET url
  → 获取 HTML
  → 使用 HTML→Markdown 转换（Go 库：github.com/JohannesKaufmann/html-to-markdown）
  → 清理无关内容（导航栏、广告、脚本等）
  → 截断到合理长度（默认 8000 字符，防止 LLM 上下文溢出）
  → 返回 Markdown 文本
```

### 4.5 模式系统 (`internal/agent/modes/`)

#### 4.5.1 模式定义

每个模式由一组配置定义：

```go
type ModeConfig struct {
    Slug             string   // 模式标识（如 "icemark", "market", "prd", "prototype"）
    Name             string   // 显示名称（如 "通用助手", "市场分析"）
    SystemPrompt     string   // 系统提示词（角色定义 + 执行步骤 + 约束条件）
    AvailableTools   []string // 该模式可用的工具列表
    WelcomeMessage   string   // 切换到该模式时的欢迎消息
}
```

#### 4.5.2 内置模式

**Icemark 模式（默认）**

```go
var IcemarkMode = ModeConfig{
    Slug: "icemark",
    Name: "通用助手",
    SystemPrompt: `你是 Icemark，一个通用智能助手，具备规划、分析、执行和审查能力。

执行流程：
1. 理解用户的目标和约束。如果有任何模糊或缺失信息，必须向用户提问确认。
2. 评估任务复杂度。
3. 简单任务直接执行，复杂任务先提出计划让用户确认。
4. 执行过程中如有疑问，随时向用户确认。

你可以使用搜索工具获取最新信息，使用网页抓取工具深入阅读内容。
在提供信息时，必须基于可靠来源，不能捏造数据。`,
    AvailableTools: []string{"web_search", "url_content_fetch"},
    WelcomeMessage: "已切换到通用助手模式。有什么我可以帮你的？",
}
```

**Market 模式**

系统提示词参考 Icemark 的 `marketModePrompt`，要求 Agent：
- 先确认分析目标、行业、目的、地理范围、时间范围
- 主动使用搜索工具收集数据
- 对搜索到的有价值链接使用 url_content_fetch 深入分析
- 应用 SWOT、PESTEL、波特五力等分析框架
- 所有数据和结论必须有来源，禁止捏造

**PRD 模式**

系统提示词参考 Icemark 的 `prdModePrompt`，要求 Agent：
- 先确认目标用户、用户目标、当前痛点
- 使用用户故事格式：「作为一个[角色]，我想要[目标]，从而[收益]」
- 使用 JTBD 理论：「当[情境]时，我想要[动机]，以便我能[预期结果]」
- 应用三问法和四种力量分析
- 生成结构化 PRD 文档

**Prototype 模式**

系统提示词参考 Icemark 的 `prototypeModePrompt`，要求 Agent：
- 确认目标平台、功能范围、UI 偏好
- 生成 HTML 原型文件
- 通过微信文件消息发送 HTML 文件

#### 4.5.3 模式切换

用户通过发送命令消息切换模式：

```
/icemark    → 切换到通用助手模式
/market     → 切换到市场分析模式
/prd        → 切换到 PRD 模式
/prototype  → 切换到原型模式
/reset      → 清空会话历史，重新开始
/mode       → 显示当前模式
/help       → 显示帮助信息
```

切换模式时：
1. 更新会话的 `mode` 字段
2. 清空对话历史（新模式需要全新的系统提示词上下文）
3. 发送欢迎消息

### 4.6 消息路由集成

#### 4.6.1 iLink 通道集成

修改 `internal/app/app.go` 中的 webhook 分发逻辑：

```go
func (s *appService) handleInboundMessage(ctx context.Context, msg ilink.WeixinMessage) {
    // 提取消息文本
    text := ilink.ExtractBodyText(msg)
    
    if s.agentEnabled && text != "" {
        sessionKey := agent.SessionKey{
            ChannelType: "ilink",
            UserID:      msg.FromUserID,
            GroupID:      msg.GroupID,
        }
        
        // 交给 Agent 处理
        go func() {
            if err := s.agent.HandleMessage(context.Background(), sessionKey, text); err != nil {
                s.logger.Error("agent handle message failed", "err", err)
            }
        }()
        return
    }
    
    // 原有 webhook 逻辑
    s.forwardToWebhook(ctx, msg)
}
```

#### 4.6.2 WeCom 通道集成

修改 `internal/app/wecom.go` 中的 `HandleInbound`：

```go
func (ws *wecomService) HandleInbound(ctx context.Context, corpID, agentID string, msg wecom.InboundMessage) {
    if ws.agentEnabled && msg.MsgType == "text" {
        sessionKey := agent.SessionKey{
            ChannelType: "wecom",
            UserID:      msg.FromUserName,
            GroupID:      "",
        }
        
        // 交给 Agent 处理
        go func() {
            if err := ws.agent.HandleMessage(context.Background(), sessionKey, msg.Content); err != nil {
                ws.logger.Error("agent handle wecom message failed", "err", err)
            }
        }()
        return
    }
    
    // 原有逻辑
}
```

#### 4.6.3 Agent 回复发送

Agent 需要一个回调接口来发送消息：

```go
type MessageSender interface {
    // SendText 发送文本消息给指定会话
    SendText(ctx context.Context, session SessionKey, text string) error
    // SendFile 发送文件消息（用于原型等）
    SendFile(ctx context.Context, session SessionKey, filename string, data []byte) error
}
```

iLink 通道的实现使用 `SendTextMessage`（需要 context_token），WeCom 通道使用企业微信 API 直接发送。

---

## 5. 数据模型

### 5.1 完整 Schema

```sql
-- 会话表
CREATE TABLE IF NOT EXISTS conversations (
    id            TEXT PRIMARY KEY,
    channel_type  TEXT NOT NULL,
    user_id       TEXT NOT NULL,
    group_id      TEXT NOT NULL DEFAULT '',
    mode          TEXT NOT NULL DEFAULT 'icemark',
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
    token_count   INTEGER DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_conv_session_key 
    ON conversations(channel_type, user_id, group_id);

-- 会话消息表
CREATE TABLE IF NOT EXISTS conversation_messages (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
    content         TEXT,
    tool_calls      TEXT,       -- JSON array
    tool_call_id    TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_conv_msg_conv_id 
    ON conversation_messages(conversation_id);

-- 工具调用日志表（用于调试和分析）
CREATE TABLE IF NOT EXISTS tool_call_logs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL,
    tool_name       TEXT NOT NULL,
    arguments       TEXT,       -- JSON
    result          TEXT,
    duration_ms     INTEGER,
    error           TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 5.2 数据清理

- 会话超过 7 天未活跃（`updated_at` 距今 > 7d）自动清理对话历史，保留会话记录
- 工具调用日志超过 30 天自动清理
- 可配置：`WCFLINK_AGENT_SESSION_TTL=168h`（默认 7 天）

---

## 6. HTTP API

### 6.1 Agent 管理接口

```
GET    /api/agent/status              → 返回 Agent 状态（是否启用、当前模式等）
GET    /api/agent/conversations       → 列出所有活跃会话
GET    /api/agent/conversations/:id   → 查看会话详情（含消息历史）
DELETE /api/agent/conversations/:id   → 删除会话
POST   /api/agent/chat               → 通过 HTTP API 直接与 Agent 对话（调试用）
```

### 6.2 HTTP 对话接口（调试/集成用）

```
POST /api/agent/chat
Content-Type: application/json

{
    "session_id": "test-session-1",      // 可选，不传则自动创建
    "message": "帮我分析低代码市场",
    "mode": "market"                      // 可选，指定模式
}

Response:
{
    "session_id": "xxx",
    "reply": "好的，我有几个问题想确认...",
    "mode": "market",
    "tool_calls_made": 0
}
```

---

## 7. 配置项汇总

```bash
# ========= Agent 开关 =========
WCFLINK_AGENT_ENABLED=true                  # 是否启用 Agent（默认 false）
WCFLINK_AGENT_DEFAULT_MODE=icemark          # 默认模式（默认 icemark）
WCFLINK_AGENT_MAX_ITERATIONS=10             # 单次消息最大 LLM 调用次数（默认 10）
WCFLINK_AGENT_SESSION_TTL=168h              # 会话过期时间（默认 7 天）

# ========= LLM 配置 =========
WCFLINK_LLM_BASE_URL=https://api.deepseek.com   # LLM API 地址
WCFLINK_LLM_API_KEY=sk-xxx                       # LLM API 密钥
WCFLINK_LLM_MODEL=deepseek-chat                  # 模型名称
WCFLINK_LLM_TEMPERATURE=0.7                      # 温度（可选）
WCFLINK_LLM_MAX_TOKENS=4096                      # 最大输出 token（可选）

# ========= 搜索配置（可选）=========
WCFLINK_SEARCH_BING_API_KEY=xxx                  # Bing Search API Key（可选）
WCFLINK_SEARCH_DEFAULT_ENGINE=duckduckgo         # 默认搜索引擎（可选）
WCFLINK_SEARCH_MAX_RESULTS=10                    # 单次搜索最大结果数（可选）

# ========= 网页抓取配置（可选）=========
WCFLINK_FETCH_MAX_CONTENT_LENGTH=8000            # 抓取内容最大字符数（可选）
WCFLINK_FETCH_TIMEOUT=30s                        # 抓取超时时间（可选）
WCFLINK_FETCH_USER_AGENT=...                     # User-Agent（可选）
```

---

## 8. 微信消息长度处理

微信单条消息有长度限制（约 2000 字符文本，实际因客户端而异）。Agent 生成的回复可能超过此限制（如市场分析报告、PRD 文档）。

### 8.1 处理策略

```go
const MaxMessageLength = 1800 // 保守值，预留 buffer

func splitMessage(text string) []string {
    if len([]rune(text)) <= MaxMessageLength {
        return []string{text}
    }
    
    // 按段落分割（优先在 \n\n 处拆分）
    // 如果单个段落超长，在 \n 处拆分
    // 如果单行超长，在 MaxMessageLength 处截断
    // 每段添加页码标记："[1/3]"
}
```

### 8.2 发送策略

- 分段消息之间间隔 500ms，防止乱序
- 第一段立即发送，后续段带延迟
- 超过 5 段的长内容，考虑生成文件发送

---

## 9. 错误处理

### 9.1 LLM 调用失败

- 网络超时：重试 2 次，间隔 2s
- API 限流（429）：等待 `Retry-After` 后重试
- API 错误（500）：重试 1 次
- 所有重试失败：给用户发送友好提示 "抱歉，AI 服务暂时不可用，请稍后再试"

### 9.2 工具调用失败

- 搜索失败：返回错误信息给 LLM，让 LLM 决定是否换个关键词重试
- 网页抓取失败：返回错误信息，LLM 可能会尝试其他 URL
- 工具超时：30 秒超时，返回超时错误

### 9.3 会话异常

- context_token 过期（iLink）：提示用户重新发送消息
- 企业微信 token 过期：自动刷新
- 对话历史损坏：自动重置会话

---

## 10. 分阶段实施计划

### 第一阶段：核心 Agent（MVP）

**目标**：跑通"收消息 → AI 回复"的完整链路，支持多轮对话和基础工具。

| 模块 | 文件 | 工作量 |
|------|------|--------|
| LLM 客户端 | `internal/llm/client.go` | 1 天 |
| 会话管理器 | `internal/agent/conversation.go` | 1 天 |
| Agent 主循环 | `internal/agent/agent.go` | 1.5 天 |
| web_search 工具 | `internal/agent/tools/web_search.go` | 1.5 天 |
| url_content_fetch 工具 | `internal/agent/tools/url_fetch.go` | 1 天 |
| 消息路由集成 | 修改 `app.go`, `wecom.go` | 0.5 天 |
| 模式系统（基础） | `internal/agent/modes/` | 0.5 天 |
| 配置集成 | 修改 `config.go` | 0.5 天 |
| 数据库 Schema | 修改 `store.go` | 0.5 天 |
| HTTP API | 修改 `server.go` | 0.5 天 |
| 消息分段发送 | `internal/agent/splitter.go` | 0.5 天 |

**预计总工时**：约 9 天

**交付物**：
- 用户通过微信发消息，Agent 能多轮对话、搜索互联网、抓取网页、返回结构化结果
- 支持 `/reset`、`/help` 命令
- 支持 DeepSeek / OpenAI Compatible 任意 LLM
- 通过 HTTP API 也能直接对话（调试用）

### 第二阶段：专业模式

**目标**：添加市场分析、PRD、原型等专业模式的系统提示词和模式切换。

| 模块 | 说明 |
|------|------|
| Market 模式 | 完整的市场分析系统提示词，参考 Icemark |
| PRD 模式 | PRD 编写系统提示词，参考 Icemark |
| Prototype 模式 | 原型生成系统提示词 + HTML 模板 |
| 模式切换命令 | `/market`、`/prd`、`/prototype`、`/icemark` |

**预计工时**：约 3 天

### 第三阶段：增强工具

**目标**：补充社交平台搜索、报告生成等高级工具。

| 模块 | 说明 |
|------|------|
| 小红书搜索 | 小红书搜索接口 |
| 知乎搜索 | 知乎搜索接口 |
| 微博搜索 | 微博搜索接口 |
| Reddit 搜索 | Reddit 搜索接口 |
| 报告生成 | 生成 Markdown 报告文件并发送 |
| 原型生成 | 生成 HTML 原型文件并发送 |
| 上下文摘要 | 长对话自动摘要压缩 |

**预计工时**：约 5 天

### 第四阶段：高级功能

**目标**：自定义模式、多 LLM 提供商切换等。

| 模块 | 说明 |
|------|------|
| 自定义模式 | 用户通过配置文件或 API 自定义模式提示词 |
| 多 LLM 支持 | 不同模式可配置不同的 LLM 提供商 |
| 用量统计 | 统计每个用户/会话的 token 消耗 |
| 用量限制 | 可配置每日/每月 token 限额 |

**预计工时**：约 4 天

---

## 11. 非功能性需求

### 11.1 性能

- LLM 调用延迟：取决于提供商，通常 2-10 秒
- 搜索工具延迟：通常 1-3 秒
- 网页抓取延迟：通常 2-5 秒
- 整体响应时间（无工具调用）：< 15 秒
- 整体响应时间（含工具调用）：< 60 秒（多轮工具调用会更长，但有"正在输入"提示）

### 11.2 可靠性

- LLM 调用失败自动重试
- 工具失败不影响 Agent 主循环（LLM 可以决定换个方法）
- 会话数据持久化到 SQLite，进程重启不丢失

### 11.3 安全

- LLM API Key 仅存储在环境变量中，不落库
- Agent 不执行任何系统命令或文件操作
- 工具的 HTTP 请求受超时限制
- 搜索和抓取内容在发送给 LLM 前截断，防止 prompt injection 风险

### 11.4 可观测性

- 所有工具调用记录到 `tool_call_logs` 表
- LLM token 消耗记录到会话
- 日志输出 Agent 处理流程的关键步骤

---

## 12. 依赖项

### 12.1 新增 Go 依赖

| 包 | 用途 | 版本 |
|----|------|------|
| `github.com/JohannesKaufmann/html-to-markdown/v2` | HTML 转 Markdown | latest stable |
| `github.com/PuerkitoBio/goquery` | HTML 解析（配合 url_fetch） | latest stable |
| `github.com/google/uuid` | 会话 ID 生成 | latest stable |

### 12.2 外部服务依赖

| 服务 | 必需 | 说明 |
|------|------|------|
| OpenAI Compatible LLM API | 是 | 需要用户提供 API Key |
| Bing Search API | 否 | 可选，提升搜索质量 |
| DuckDuckGo | 否 | 免费，作为默认搜索引擎 |

---

## 13. 项目文件结构（新增部分）

```
wcfLink/
├── internal/
│   ├── llm/                        # LLM 客户端
│   │   └── client.go               # OpenAI Compatible 客户端实现
│   ├── agent/                      # Agent 引擎
│   │   ├── agent.go                # Agent 主循环
│   │   ├── conversation.go         # 会话管理器
│   │   ├── splitter.go             # 消息分段
│   │   ├── modes/                  # 模式定义
│   │   │   ├── modes.go            # 模式注册和管理
│   │   │   ├── icemark.go          # 通用助手模式
│   │   │   ├── market.go           # 市场分析模式
│   │   │   ├── prd.go              # PRD 模式
│   │   │   └── prototype.go        # 原型模式
│   │   └── tools/                  # 工具实现
│   │       ├── registry.go         # 工具注册表
│   │       ├── web_search.go       # 互联网搜索
│   │       ├── url_fetch.go        # 网页内容抓取
│   │       ├── social_search.go    # 社交平台搜索（第二阶段）
│   │       └── prototype_gen.go    # 原型生成（第二阶段）
│   ├── app/
│   │   ├── app.go                  # [修改] 集成 Agent 路由
│   │   └── wecom.go                # [修改] 集成 Agent 路由
│   ├── config/
│   │   └── config.go               # [修改] 新增 Agent/LLM 配置项
│   ├── httpapi/
│   │   └── server.go               # [修改] 新增 Agent HTTP API
│   └── store/
│       └── store.go                # [修改] 新增会话相关 Schema
├── docs/
│   └── prd-agent-feature.md        # 本文档
└── README.md                       # [修改] 新增 Agent 功能文档
```
