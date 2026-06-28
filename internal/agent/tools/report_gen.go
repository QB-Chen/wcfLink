package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QB-Chen/wcfLink/internal/llm"
)

type ReportGenTool struct{}

func NewReportGenTool() *ReportGenTool {
	return &ReportGenTool{}
}

func (t *ReportGenTool) Name() string { return "generate_report" }

func (t *ReportGenTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "generate_report",
			Description: "生成结构化 Markdown 报告。将收集到的信息整理成带标题、章节的完整报告格式。适用于市场分析报告、调研报告、竞品分析等场景。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "报告标题",
					},
					"sections": map[string]interface{}{
						"type":        "array",
						"description": "报告章节列表",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"heading": map[string]interface{}{
									"type":        "string",
									"description": "章节标题",
								},
								"content": map[string]interface{}{
									"type":        "string",
									"description": "章节内容（Markdown 格式）",
								},
							},
							"required": []string{"heading", "content"},
						},
					},
					"summary": map[string]interface{}{
						"type":        "string",
						"description": "报告摘要/执行总结（可选）",
					},
					"sources": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "参考来源 URL 列表（可选）",
					},
				},
				"required": []string{"title", "sections"},
			},
		},
	}
}

type reportSection struct {
	Heading string `json:"heading"`
	Content string `json:"content"`
}

type reportGenArgs struct {
	Title    string          `json:"title"`
	Sections []reportSection `json:"sections"`
	Summary  string          `json:"summary"`
	Sources  []string        `json:"sources"`
}

func (t *ReportGenTool) Execute(_ context.Context, arguments string) (string, error) {
	var args reportGenArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Title == "" {
		return "", fmt.Errorf("title is required")
	}
	if len(args.Sections) == 0 {
		return "", fmt.Errorf("at least one section is required")
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", args.Title))

	if args.Summary != "" {
		sb.WriteString("## 执行摘要\n\n")
		sb.WriteString(args.Summary)
		sb.WriteString("\n\n---\n\n")
	}

	for i, section := range args.Sections {
		sb.WriteString(fmt.Sprintf("## %d. %s\n\n", i+1, section.Heading))
		sb.WriteString(section.Content)
		sb.WriteString("\n\n")
	}

	if len(args.Sources) > 0 {
		sb.WriteString("---\n\n## 参考来源\n\n")
		for i, src := range args.Sources {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, src))
		}
		sb.WriteString("\n")
	}

	report := strings.TrimSpace(sb.String())
	return fmt.Sprintf("报告已生成（共 %d 个章节，%d 字）：\n\n%s", len(args.Sections), len([]rune(report)), report), nil
}
