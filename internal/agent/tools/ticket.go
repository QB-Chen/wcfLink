package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QB-Chen/wcfLink/internal/agent/support"
	"github.com/QB-Chen/wcfLink/internal/llm"
)

// --- Ticket Create ---

type TicketCreateTool struct {
	store *support.Store
}

func NewTicketCreateTool(store *support.Store) *TicketCreateTool {
	return &TicketCreateTool{store: store}
}

func (t *TicketCreateTool) Name() string { return "ticket_create" }

func (t *TicketCreateTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "ticket_create",
			Description: "创建客服工单，记录客户问题以便跟踪处理。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"subject": map[string]interface{}{
						"type":        "string",
						"description": "工单标题/主题",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "问题详细描述",
					},
					"customer_id": map[string]interface{}{
						"type":        "string",
						"description": "客户 ID（微信用户 ID）",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "medium", "high", "critical"},
						"description": "优先级（默认 medium）",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "问题分类（如：退款、Bug、咨询、投诉）",
					},
				},
				"required": []string{"subject"},
			},
		},
	}
}

type ticketCreateArgs struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	CustomerID  string `json:"customer_id"`
	Priority    string `json:"priority"`
	Category    string `json:"category"`
}

func (t *TicketCreateTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args ticketCreateArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Subject == "" {
		return "", fmt.Errorf("subject is required")
	}

	ticket, err := t.store.TicketCreate(ctx, support.Ticket{
		Subject:     args.Subject,
		Description: args.Description,
		CustomerID:  args.CustomerID,
		Priority:    args.Priority,
		Category:    args.Category,
	})
	if err != nil {
		return fmt.Sprintf("创建工单失败: %v", err), nil
	}

	return fmt.Sprintf("工单已创建成功。\n工单 ID: %s\n主题: %s\n优先级: %s\n状态: %s",
		ticket.ID, ticket.Subject, ticket.Priority, ticket.Status), nil
}

// --- Ticket Query ---

type TicketQueryTool struct {
	store *support.Store
}

func NewTicketQueryTool(store *support.Store) *TicketQueryTool {
	return &TicketQueryTool{store: store}
}

func (t *TicketQueryTool) Name() string { return "ticket_query" }

func (t *TicketQueryTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "ticket_query",
			Description: "查询工单列表或指定工单详情。可按状态、优先级、客户 ID 等条件过滤。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ticket_id": map[string]interface{}{
						"type":        "string",
						"description": "查询指定工单 ID 的详情（与过滤条件互斥）",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"open", "in_progress", "pending", "resolved", "closed"},
						"description": "按状态过滤",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "medium", "high", "critical"},
						"description": "按优先级过滤",
					},
					"customer_id": map[string]interface{}{
						"type":        "string",
						"description": "按客户 ID 过滤",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "返回数量（默认 10）",
					},
				},
			},
		},
	}
}

type ticketQueryArgs struct {
	TicketID   string `json:"ticket_id"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	CustomerID string `json:"customer_id"`
	Limit      int    `json:"limit"`
}

func (t *TicketQueryTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args ticketQueryArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if args.TicketID != "" {
		ticket, err := t.store.TicketGet(ctx, args.TicketID)
		if err != nil {
			return fmt.Sprintf("工单 %s 未找到", args.TicketID), nil
		}
		return formatTicket(ticket), nil
	}

	if args.Limit <= 0 {
		args.Limit = 10
	}
	filters := make(map[string]string)
	if args.Status != "" {
		filters["status"] = args.Status
	}
	if args.Priority != "" {
		filters["priority"] = args.Priority
	}
	if args.CustomerID != "" {
		filters["customer_id"] = args.CustomerID
	}

	tickets, err := t.store.TicketQuery(ctx, filters, args.Limit)
	if err != nil {
		return fmt.Sprintf("查询工单失败: %v", err), nil
	}
	if len(tickets) == 0 {
		return "未找到符合条件的工单。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个工单：\n\n", len(tickets)))
	for _, ticket := range tickets {
		sb.WriteString(formatTicketBrief(ticket))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// --- Ticket Update ---

type TicketUpdateTool struct {
	store *support.Store
}

func NewTicketUpdateTool(store *support.Store) *TicketUpdateTool {
	return &TicketUpdateTool{store: store}
}

func (t *TicketUpdateTool) Name() string { return "ticket_update" }

func (t *TicketUpdateTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "ticket_update",
			Description: "更新工单状态、优先级、备注等信息。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"ticket_id": map[string]interface{}{
						"type":        "string",
						"description": "工单 ID",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"open", "in_progress", "pending", "resolved", "closed"},
						"description": "更新状态",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"low", "medium", "high", "critical"},
						"description": "更新优先级",
					},
					"notes": map[string]interface{}{
						"type":        "string",
						"description": "追加备注",
					},
					"assignee": map[string]interface{}{
						"type":        "string",
						"description": "指派处理人",
					},
				},
				"required": []string{"ticket_id"},
			},
		},
	}
}

type ticketUpdateArgs struct {
	TicketID string `json:"ticket_id"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Notes    string `json:"notes"`
	Assignee string `json:"assignee"`
}

func (t *TicketUpdateTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args ticketUpdateArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.TicketID == "" {
		return "", fmt.Errorf("ticket_id is required")
	}

	updates := make(map[string]interface{})
	if args.Status != "" {
		updates["status"] = args.Status
	}
	if args.Priority != "" {
		updates["priority"] = args.Priority
	}
	if args.Notes != "" {
		updates["notes"] = args.Notes
	}
	if args.Assignee != "" {
		updates["assignee"] = args.Assignee
	}

	if len(updates) == 0 {
		return "未指定需要更新的字段。", nil
	}

	if err := t.store.TicketUpdate(ctx, args.TicketID, updates); err != nil {
		return fmt.Sprintf("更新工单失败: %v", err), nil
	}

	var parts []string
	for k, v := range updates {
		parts = append(parts, fmt.Sprintf("%s → %v", k, v))
	}
	return fmt.Sprintf("工单 %s 已更新：%s", args.TicketID, strings.Join(parts, ", ")), nil
}

func formatTicket(t support.Ticket) string {
	s := fmt.Sprintf("工单 ID: %s\n主题: %s\n状态: %s\n优先级: %s\n分类: %s\n客户: %s\n描述: %s\n创建时间: %s\n更新时间: %s",
		t.ID, t.Subject, t.Status, t.Priority, t.Category, t.CustomerID, t.Description,
		t.CreatedAt.Format("2006-01-02 15:04:05"), t.UpdatedAt.Format("2006-01-02 15:04:05"))
	if t.Assignee != "" {
		s += fmt.Sprintf("\n处理人: %s", t.Assignee)
	}
	if t.Notes != "" {
		s += fmt.Sprintf("\n备注: %s", t.Notes)
	}
	if t.ClosedAt != nil {
		s += fmt.Sprintf("\n关闭时间: %s", t.ClosedAt.Format("2006-01-02 15:04:05"))
	}
	return s
}

func formatTicketBrief(t support.Ticket) string {
	return fmt.Sprintf("- [%s] %s | 状态: %s | 优先级: %s | 客户: %s | %s",
		t.ID[:8], t.Subject, t.Status, t.Priority, t.CustomerID,
		t.CreatedAt.Format("01-02 15:04"))
}
