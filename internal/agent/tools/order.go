package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QB-Chen/wcfLink/internal/agent/support"
	"github.com/QB-Chen/wcfLink/internal/llm"
)

// --- Order Query ---

type OrderQueryTool struct {
	store *support.Store
}

func NewOrderQueryTool(store *support.Store) *OrderQueryTool {
	return &OrderQueryTool{store: store}
}

func (t *OrderQueryTool) Name() string { return "order_query" }

func (t *OrderQueryTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "order_query",
			Description: "查询订单信息。可按订单 ID 查详情，或按客户 ID、状态等条件查列表。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"order_id": map[string]interface{}{
						"type":        "string",
						"description": "查询指定订单 ID 的详情",
					},
					"customer_id": map[string]interface{}{
						"type":        "string",
						"description": "按客户 ID 查询订单列表",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"pending", "paid", "shipped", "delivered", "refunded", "cancelled"},
						"description": "按状态过滤",
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

type orderQueryArgs struct {
	OrderID    string `json:"order_id"`
	CustomerID string `json:"customer_id"`
	Status     string `json:"status"`
	Limit      int    `json:"limit"`
}

func (t *OrderQueryTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args orderQueryArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}

	if args.OrderID != "" {
		order, err := t.store.OrderGet(ctx, args.OrderID)
		if err != nil {
			return fmt.Sprintf("订单 %s 未找到", args.OrderID), nil
		}
		return formatOrder(order), nil
	}

	if args.Limit <= 0 {
		args.Limit = 10
	}
	filters := make(map[string]string)
	if args.CustomerID != "" {
		filters["customer_id"] = args.CustomerID
	}
	if args.Status != "" {
		filters["status"] = args.Status
	}

	orders, err := t.store.OrderQuery(ctx, filters, args.Limit)
	if err != nil {
		return fmt.Sprintf("查询订单失败: %v", err), nil
	}
	if len(orders) == 0 {
		return "未找到符合条件的订单。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个订单：\n\n", len(orders)))
	for _, order := range orders {
		sb.WriteString(formatOrderBrief(order))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// --- Order Create ---

type OrderCreateTool struct {
	store *support.Store
}

func NewOrderCreateTool(store *support.Store) *OrderCreateTool {
	return &OrderCreateTool{store: store}
}

func (t *OrderCreateTool) Name() string { return "order_create" }

func (t *OrderCreateTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "order_create",
			Description: "创建新订单记录。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"customer_id": map[string]interface{}{
						"type":        "string",
						"description": "客户 ID",
					},
					"product": map[string]interface{}{
						"type":        "string",
						"description": "产品名称",
					},
					"amount": map[string]interface{}{
						"type":        "number",
						"description": "金额",
					},
					"currency": map[string]interface{}{
						"type":        "string",
						"description": "货币（默认 CNY）",
					},
					"payment_method": map[string]interface{}{
						"type":        "string",
						"description": "支付方式",
					},
					"notes": map[string]interface{}{
						"type":        "string",
						"description": "备注",
					},
				},
				"required": []string{"product", "amount"},
			},
		},
	}
}

type orderCreateArgs struct {
	CustomerID    string  `json:"customer_id"`
	Product       string  `json:"product"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	PaymentMethod string  `json:"payment_method"`
	Notes         string  `json:"notes"`
}

func (t *OrderCreateTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args orderCreateArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Product == "" {
		return "", fmt.Errorf("product is required")
	}

	order, err := t.store.OrderCreate(ctx, support.Order{
		CustomerID:    args.CustomerID,
		Product:       args.Product,
		Amount:        args.Amount,
		Currency:      args.Currency,
		PaymentMethod: args.PaymentMethod,
		Notes:         args.Notes,
	})
	if err != nil {
		return fmt.Sprintf("创建订单失败: %v", err), nil
	}

	return fmt.Sprintf("订单已创建成功。\n订单 ID: %s\n产品: %s\n金额: %.2f %s\n状态: %s",
		order.ID, order.Product, order.Amount, order.Currency, order.Status), nil
}

// --- Order Refund ---

type OrderRefundTool struct {
	store *support.Store
}

func NewOrderRefundTool(store *support.Store) *OrderRefundTool {
	return &OrderRefundTool{store: store}
}

func (t *OrderRefundTool) Name() string { return "order_refund" }

func (t *OrderRefundTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "order_refund",
			Description: "处理订单退款。需要提供订单 ID、退款金额和退款原因。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"order_id": map[string]interface{}{
						"type":        "string",
						"description": "订单 ID",
					},
					"amount": map[string]interface{}{
						"type":        "number",
						"description": "退款金额",
					},
					"reason": map[string]interface{}{
						"type":        "string",
						"description": "退款原因",
					},
				},
				"required": []string{"order_id", "amount", "reason"},
			},
		},
	}
}

type orderRefundArgs struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
	Reason  string  `json:"reason"`
}

func (t *OrderRefundTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args orderRefundArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.OrderID == "" {
		return "", fmt.Errorf("order_id is required")
	}
	if args.Reason == "" {
		return "", fmt.Errorf("reason is required")
	}

	order, err := t.store.OrderGet(ctx, args.OrderID)
	if err != nil {
		return fmt.Sprintf("订单 %s 未找到", args.OrderID), nil
	}

	if order.Status == "refunded" {
		return fmt.Sprintf("订单 %s 已经退款过了（退款金额: %.2f %s）", args.OrderID, order.RefundAmount, order.Currency), nil
	}

	if args.Amount > order.Amount {
		return fmt.Sprintf("退款金额 %.2f 超过订单金额 %.2f", args.Amount, order.Amount), nil
	}

	if err := t.store.OrderRefund(ctx, args.OrderID, args.Amount, args.Reason); err != nil {
		return fmt.Sprintf("退款处理失败: %v", err), nil
	}

	return fmt.Sprintf("退款已处理成功。\n订单 ID: %s\n退款金额: %.2f %s\n原因: %s",
		args.OrderID, args.Amount, order.Currency, args.Reason), nil
}

func formatOrder(o support.Order) string {
	s := fmt.Sprintf("订单 ID: %s\n客户: %s\n产品: %s\n金额: %.2f %s\n状态: %s\n支付方式: %s\n创建时间: %s",
		o.ID, o.CustomerID, o.Product, o.Amount, o.Currency, o.Status, o.PaymentMethod,
		o.CreatedAt.Format("2006-01-02 15:04:05"))
	if o.Notes != "" {
		s += fmt.Sprintf("\n备注: %s", o.Notes)
	}
	if o.RefundAmount > 0 {
		s += fmt.Sprintf("\n退款金额: %.2f %s\n退款原因: %s", o.RefundAmount, o.Currency, o.RefundReason)
		if o.RefundedAt != nil {
			s += fmt.Sprintf("\n退款时间: %s", o.RefundedAt.Format("2006-01-02 15:04:05"))
		}
	}
	return s
}

func formatOrderBrief(o support.Order) string {
	s := fmt.Sprintf("- [%s] %s | %.2f %s | 状态: %s | 客户: %s | %s",
		o.ID[:8], o.Product, o.Amount, o.Currency, o.Status, o.CustomerID,
		o.CreatedAt.Format("01-02 15:04"))
	return s
}
