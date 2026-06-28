package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/QB-Chen/wcfLink/internal/agent/modes"
	"github.com/QB-Chen/wcfLink/internal/agent/support"
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
	DefaultMode     string
	MaxIterations   int
	SessionTTL      time.Duration
	Temperature     *float64
	MaxTokens       int
	FetchMaxContent int
}

type Agent struct {
	llmClient      *llm.Client
	convMgr        *ConversationManager
	toolRegistry   *tools.Registry
	sender         MessageSender
	logger         *slog.Logger
	config         AgentConfig
	supportStore   *support.Store
	supportBuilder *support.Builder
}

func New(llmClient *llm.Client, convMgr *ConversationManager, sender MessageSender, logger *slog.Logger, cfg AgentConfig, supportSt *support.Store) *Agent {
	if cfg.DefaultMode == "" {
		cfg.DefaultMode = defaultMode
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = defaultMaxIterations
	}

	registry := tools.NewRegistry()
	registry.Register(tools.NewWebSearchTool())
	registry.Register(tools.NewURLFetchTool(cfg.FetchMaxContent))
	registry.Register(tools.NewSocialSearchTool())
	registry.Register(tools.NewReportGenTool())
	registry.Register(tools.NewPrototypeGenTool())

	if supportSt != nil {
		registry.Register(tools.NewKBSearchTool(supportSt))
		registry.Register(tools.NewTicketCreateTool(supportSt))
		registry.Register(tools.NewTicketQueryTool(supportSt))
		registry.Register(tools.NewTicketUpdateTool(supportSt))
		registry.Register(tools.NewOrderQueryTool(supportSt))
		registry.Register(tools.NewOrderCreateTool(supportSt))
		registry.Register(tools.NewOrderRefundTool(supportSt))
	}

	var builder *support.Builder
	if supportSt != nil {
		builder = support.NewBuilder(llmClient, supportSt)
	}

	return &Agent{
		llmClient:      llmClient,
		convMgr:        convMgr,
		toolRegistry:   registry,
		sender:         sender,
		logger:         logger,
		config:         cfg,
		supportStore:   supportSt,
		supportBuilder: builder,
	}
}

func NewWithSender(base *Agent, sender MessageSender) *Agent {
	return &Agent{
		llmClient:      base.llmClient,
		convMgr:        base.convMgr,
		toolRegistry:   base.toolRegistry,
		sender:         sender,
		logger:         base.logger,
		config:         base.config,
		supportStore:   base.supportStore,
		supportBuilder: base.supportBuilder,
	}
}

func (a *Agent) HandleMessage(ctx context.Context, session SessionKey, userMessage string) error {
	lock := a.convMgr.GetSessionLock(session)
	lock.Lock()
	defer lock.Unlock()

	if cmd := parseCommand(userMessage); cmd != "" {
		return a.handleCommand(ctx, session, cmd, userMessage)
	}

	conv, err := a.convMgr.GetOrCreate(ctx, session, a.config.DefaultMode)
	if err != nil {
		return a.sendError(ctx, session, fmt.Errorf("获取会话失败: %w", err))
	}

	if conv.Mode == "support-setup" && a.supportBuilder != nil {
		return a.handleSetupMessage(ctx, session, conv, userMessage)
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

	systemPrompt := mode.SystemPrompt
	if conv.Mode == "support" && a.supportStore != nil {
		if profile, err := a.supportStore.ProfileGetDefault(ctx); err == nil {
			systemPrompt = support.GenerateCustomPrompt(systemPrompt, profile)
		}
	}

	for iteration := 0; iteration < a.config.MaxIterations; iteration++ {
		history, err := a.convMgr.GetMessages(ctx, conv.ID)
		if err != nil {
			return a.sendError(ctx, session, fmt.Errorf("获取对话历史失败: %w", err))
		}

		history = compactHistory(ctx, a.llmClient, systemPrompt, history, a.config.Temperature, a.config.MaxTokens)

		messages := make([]llm.Message, 0, len(history)+1)
		messages = append(messages, llm.Message{Role: llm.RoleSystem, Content: systemPrompt})
		messages = append(messages, history...)

		toolDefs := a.toolRegistry.Definitions(mode.AvailableTools)

		req := llm.ChatRequest{
			Messages:    messages,
			Tools:       toolDefs,
			Temperature: a.config.Temperature,
			MaxTokens:   a.config.MaxTokens,
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

func (a *Agent) handleCommand(ctx context.Context, session SessionKey, cmd string, rawText string) error {
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

	case "support-setup":
		return a.handleSupportSetup(ctx, session)

	case "support-profiles":
		return a.handleSupportProfiles(ctx, session)

	case "support-use":
		return a.handleSupportUse(ctx, session, rawText)

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
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(fields[0])
}

func (a *Agent) handleSupportSetup(ctx context.Context, session SessionKey) error {
	if a.supportBuilder == nil {
		return a.sendReply(ctx, session, "客服支持模块未启用。")
	}
	conv, err := a.convMgr.GetOrCreate(ctx, session, a.config.DefaultMode)
	if err != nil {
		return a.sendError(ctx, session, err)
	}
	if err := a.convMgr.UpdateMode(ctx, conv.ID, "support-setup"); err != nil {
		return a.sendError(ctx, session, err)
	}
	if err := a.convMgr.ClearMessages(ctx, conv.ID); err != nil {
		return a.sendError(ctx, session, err)
	}
	return a.sendReply(ctx, session, "已进入客服规范配置向导。\n\n请告诉我你想配置什么类型的客服规范？比如：\n- 电商客服\n- SaaS 技术支持\n- 教育咨询\n- 金融理财\n\n或者直接描述你的业务类型和需求。\n\n（输入 /reset 可退出配置向导）")
}

func (a *Agent) handleSetupMessage(ctx context.Context, session SessionKey, conv Conversation, userMessage string) error {
	history, err := a.convMgr.GetMessages(ctx, conv.ID)
	if err != nil {
		return a.sendError(ctx, session, fmt.Errorf("获取对话历史失败: %w", err))
	}

	if err := a.convMgr.AddMessage(ctx, conv.ID, llm.Message{Role: llm.RoleUser, Content: userMessage}); err != nil {
		return err
	}

	reply, profile, err := a.supportBuilder.ProcessSetupMessage(ctx, history, userMessage, a.config.Temperature, a.config.MaxTokens)
	if err != nil {
		return a.sendError(ctx, session, err)
	}

	if err := a.convMgr.AddMessage(ctx, conv.ID, llm.Message{Role: llm.RoleAssistant, Content: reply}); err != nil {
		return err
	}

	if profile != nil {
		if err := a.convMgr.UpdateMode(ctx, conv.ID, "support"); err != nil {
			return a.sendError(ctx, session, err)
		}
		if err := a.convMgr.ClearMessages(ctx, conv.ID); err != nil {
			return a.sendError(ctx, session, err)
		}
	}

	return a.sendReply(ctx, session, reply)
}

func (a *Agent) handleSupportProfiles(ctx context.Context, session SessionKey) error {
	if a.supportStore == nil {
		return a.sendReply(ctx, session, "客服支持模块未启用。")
	}
	profiles, err := a.supportStore.ProfileList(ctx)
	if err != nil {
		return a.sendError(ctx, session, err)
	}
	if len(profiles) == 0 {
		return a.sendReply(ctx, session, "暂无客服规范配置。\n使用 /support-setup 创建一个。")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 个客服规范配置：\n\n", len(profiles)))
	for _, p := range profiles {
		defaultMark := ""
		if p.IsDefault {
			defaultMark = " ★ 默认"
		}
		sb.WriteString(fmt.Sprintf("• %s%s\n  公司: %s | 行业: %s\n", p.Name, defaultMark, p.CompanyName, p.Industry))
	}
	sb.WriteString("\n使用 /support-use <名称> 切换默认规范")
	return a.sendReply(ctx, session, sb.String())
}

func (a *Agent) handleSupportUse(ctx context.Context, session SessionKey, rawText string) error {
	if a.supportStore == nil {
		return a.sendReply(ctx, session, "客服支持模块未启用。")
	}
	parts := strings.SplitN(strings.TrimSpace(rawText), " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		return a.sendReply(ctx, session, "请使用 /support-use <配置名称> 来切换客服规范。\n例如: /support-use 电商客服")
	}
	profileName := strings.TrimSpace(parts[1])

	profile, err := a.supportStore.ProfileGetByName(ctx, profileName)
	if err != nil {
		return a.sendReply(ctx, session, fmt.Sprintf("未找到名为「%s」的客服规范配置。\n使用 /support-profiles 查看所有配置。", profileName))
	}

	if err := a.supportStore.ProfileSetDefault(ctx, profile.ID); err != nil {
		return a.sendError(ctx, session, err)
	}
	return a.sendReply(ctx, session, fmt.Sprintf("已将「%s」设为默认客服规范。\n使用 /support 切换到客服模式即可生效。", profileName))
}

func (a *Agent) SupportStore() *support.Store {
	return a.supportStore
}

func helpText() string {
	return `可用命令：
/icemark          — 切换到通用助手模式
/market           — 切换到市场分析模式
/prd              — 切换到 PRD 模式
/prototype        — 切换到原型设计模式
/support          — 切换到客服助手模式
/support-setup    — 启动客服规范配置向导（Builder）
/support-profiles — 查看所有客服规范配置
/support-use <名称> — 切换默认客服规范
/reset            — 清空当前会话历史
/mode             — 查看当前模式
/help             — 显示帮助信息

直接发送消息即可开始对话。`
}

func (a *Agent) ConversationManager() *ConversationManager {
	return a.convMgr
}
