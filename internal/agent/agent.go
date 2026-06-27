package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/QB-Chen/wcfLink/internal/agent/modes"
	"github.com/QB-Chen/wcfLink/internal/agent/tools"
	"github.com/QB-Chen/wcfLink/internal/llm"
)

const (
	defaultMaxIterations = 10
	defaultMode          = "icemark"
)

type MessageSender interface {
	SendText(ctx context.Context, session SessionKey, text string) error
}

type AgentConfig struct {
	DefaultMode   string
	MaxIterations int
	SessionTTL    time.Duration
}

type Agent struct {
	llmClient    *llm.Client
	convMgr      *ConversationManager
	toolRegistry *tools.Registry
	sender       MessageSender
	logger       *slog.Logger
	config       AgentConfig
}

func New(llmClient *llm.Client, convMgr *ConversationManager, sender MessageSender, logger *slog.Logger, cfg AgentConfig) *Agent {
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = defaultMode
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = defaultMaxIterations
	}

	registry := tools.NewRegistry()
	registry.Register(tools.NewWebSearchTool())
	registry.Register(tools.NewURLFetchTool(0))

	return &Agent{
		llmClient:    llmClient,
		convMgr:      convMgr,
		toolRegistry: registry,
		sender:       sender,
		logger:       logger,
		config:       cfg,
	}
}

func NewWithSender(base *Agent, sender MessageSender) *Agent {
	return &Agent{
		llmClient:    base.llmClient,
		convMgr:      base.convMgr,
		toolRegistry: base.toolRegistry,
		sender:       sender,
		logger:       base.logger,
		config:       base.config,
	}
}

func (a *Agent) HandleMessage(ctx context.Context, session SessionKey, userMessage string) error {
	lock := a.convMgr.GetSessionLock(session)
	lock.Lock()
	defer lock.Unlock()

	if cmd := parseCommand(userMessage); cmd != "" {
		return a.handleCommand(ctx, session, cmd)
	}

	conv, err := a.convMgr.GetOrCreate(ctx, session, a.config.DefaultMode)
	if err != nil {
		return a.sendError(ctx, session, fmt.Errorf("获取会话失败: %w", err))
	}

	if err := a.convMgr.AddMessage(ctx, conv.ID, llm.Message{Role: llm.RoleUser, Content: userMessage}); err != nil {
		return err
	}
	if err := a.convMgr.TouchUpdatedAt(ctx, conv.ID); err != nil {
		return err
	}

	mode, ok := modes.Get(conv.Mode)
	if !ok {
		mode = modes.IcemarkMode
	}

	for iteration := 0; iteration < a.config.MaxIterations; iteration++ {
		history, err := a.convMgr.GetMessages(ctx, conv.ID)
		if err != nil {
			return a.sendError(ctx, session, fmt.Errorf("获取对话历史失败: %w", err))
		}

		messages := make([]llm.Message, 0, len(history)+1)
		messages = append(messages, llm.Message{Role: llm.RoleSystem, Content: mode.SystemPrompt})
		messages = append(messages, history...)

		toolDefs := a.toolRegistry.Definitions(mode.AvailableTools)

		req := llm.ChatRequest{
			Messages: messages,
			Tools:    toolDefs,
		}

		a.logger.Debug("agent calling llm",
			"session", session.String(),
			"mode", conv.Mode,
			"iteration", iteration,
			"message_count", len(messages),
		)

		resp, err := a.llmClient.Chat(ctx, req)
		if err != nil {
			return a.sendError(ctx, session, fmt.Errorf("AI 服务暂时不可用，请稍后再试: %w", err))
		}

		assistantMsg := resp.Choices[0].Message
		if err := a.convMgr.AddMessage(ctx, conv.ID, assistantMsg); err != nil {
			return err
		}

		finishReason := resp.Choices[0].FinishReason

		if finishReason == "tool_calls" && len(assistantMsg.ToolCalls) > 0 {
			for _, tc := range assistantMsg.ToolCalls {
				a.logger.Info("agent executing tool",
					"session", session.String(),
					"tool", tc.Function.Name,
					"arguments", tc.Function.Arguments,
				)

				start := time.Now()
				result, toolErr := a.executeTool(ctx, tc)
				duration := time.Since(start).Milliseconds()

				errText := ""
				if toolErr != nil {
					errText = toolErr.Error()
					result = fmt.Sprintf("工具调用失败: %s", errText)
				}

				_ = a.convMgr.LogToolCall(ctx, conv.ID, tc.Function.Name, tc.Function.Arguments, result, errText, duration)

				toolMsg := llm.Message{
					Role:       llm.RoleTool,
					Content:    result,
					ToolCallID: tc.ID,
				}
				if err := a.convMgr.AddMessage(ctx, conv.ID, toolMsg); err != nil {
					return err
				}
			}
			continue
		}

		if assistantMsg.Content != "" {
			return a.sendReply(ctx, session, assistantMsg.Content)
		}
		return nil
	}

	return a.sendReply(ctx, session, "抱歉，处理过程中超过了最大迭代次数，请重试或简化你的需求。")
}

func (a *Agent) executeTool(ctx context.Context, tc llm.ToolCall) (string, error) {
	tool, ok := a.toolRegistry.Get(tc.Function.Name)
	if !ok {
		return "", fmt.Errorf("未知工具: %s", tc.Function.Name)
	}
	return tool.Execute(ctx, tc.Function.Arguments)
}

func (a *Agent) handleCommand(ctx context.Context, session SessionKey, cmd string) error {
	switch cmd {
	case "reset":
		conv, err := a.convMgr.GetOrCreate(ctx, session, a.config.DefaultMode)
		if err != nil {
			return a.sendError(ctx, session, err)
		}
		if err := a.convMgr.ClearMessages(ctx, conv.ID); err != nil {
			return a.sendError(ctx, session, err)
		}
		return a.sendReply(ctx, session, "会话已重置。")

	case "mode":
		conv, err := a.convMgr.GetOrCreate(ctx, session, a.config.DefaultMode)
		if err != nil {
			return a.sendError(ctx, session, err)
		}
		mode, ok := modes.Get(conv.Mode)
		if !ok {
			return a.sendReply(ctx, session, fmt.Sprintf("当前模式: %s", conv.Mode))
		}
		return a.sendReply(ctx, session, fmt.Sprintf("当前模式: %s (%s)", mode.Name, mode.Slug))

	case "help":
		return a.sendReply(ctx, session, helpText())

	default:
		if modeConfig, ok := modes.Get(cmd); ok {
			conv, err := a.convMgr.GetOrCreate(ctx, session, a.config.DefaultMode)
			if err != nil {
				return a.sendError(ctx, session, err)
			}
			if err := a.convMgr.UpdateMode(ctx, conv.ID, cmd); err != nil {
				return a.sendError(ctx, session, err)
			}
			if err := a.convMgr.ClearMessages(ctx, conv.ID); err != nil {
				return a.sendError(ctx, session, err)
			}
			return a.sendReply(ctx, session, modeConfig.WelcomeMessage)
		}
		return a.sendReply(ctx, session, fmt.Sprintf("未知命令: /%s\n\n%s", cmd, helpText()))
	}
}

func (a *Agent) sendReply(ctx context.Context, session SessionKey, text string) error {
	segments := SplitMessage(text)
	for i, seg := range segments {
		if err := a.sender.SendText(ctx, session, seg); err != nil {
			a.logger.Error("agent send reply failed", "session", session.String(), "segment", i, "err", err)
			return err
		}
		if i < len(segments)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

func (a *Agent) sendError(ctx context.Context, session SessionKey, err error) error {
	a.logger.Error("agent error", "session", session.String(), "err", err)
	_ = a.sender.SendText(ctx, session, fmt.Sprintf("处理出错: %v", err))
	return err
}

func parseCommand(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	cmd := strings.TrimPrefix(text, "/")
	cmd = strings.Fields(cmd)[0]
	return strings.ToLower(cmd)
}

func helpText() string {
	return `可用命令：
/icemark    — 切换到通用助手模式
/market     — 切换到市场分析模式
/prd        — 切换到 PRD 模式
/prototype  — 切换到原型设计模式
/reset      — 清空当前会话历史
/mode       — 查看当前模式
/help       — 显示帮助信息

直接发送消息即可开始对话。`
}

func (a *Agent) ConversationManager() *ConversationManager {
	return a.convMgr
}
