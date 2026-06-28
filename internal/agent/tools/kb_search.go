package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QB-Chen/wcfLink/internal/agent/support"
	"github.com/QB-Chen/wcfLink/internal/llm"
)

type KBSearchTool struct {
	store *support.Store
}

func NewKBSearchTool(store *support.Store) *KBSearchTool {
	return &KBSearchTool{store: store}
}

func (t *KBSearchTool) Name() string { return "kb_search" }

func (t *KBSearchTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "kb_search",
			Description: "搜索知识库（FAQ/产品文档），根据关键词匹配相关的问答条目。优先使用此工具回答客户问题。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "搜索关键词",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "按分类过滤（可选）",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "返回结果数量（默认 5）",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

type kbSearchArgs struct {
	Query    string `json:"query"`
	Category string `json:"category"`
	Limit    int    `json:"limit"`
}

func (t *KBSearchTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args kbSearchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	if args.Limit <= 0 {
		args.Limit = 5
	}

	articles, err := t.store.KBSearch(ctx, args.Query, args.Category, args.Limit)
	if err != nil {
		return fmt.Sprintf("知识库搜索失败: %v", err), nil
	}

	if len(articles) == 0 {
		return "未在知识库中找到相关内容。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 条相关知识库条目：\n\n", len(articles)))
	for i, a := range articles {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, a.Question))
		if a.Category != "" {
			sb.WriteString(fmt.Sprintf("分类: %s\n", a.Category))
		}
		sb.WriteString(fmt.Sprintf("回答: %s\n\n", a.Answer))
	}
	return sb.String(), nil
}
