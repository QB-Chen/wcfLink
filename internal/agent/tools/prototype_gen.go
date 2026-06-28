package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QB-Chen/wcfLink/internal/llm"
)

type PrototypeGenTool struct{}

func NewPrototypeGenTool() *PrototypeGenTool {
	return &PrototypeGenTool{}
}

func (t *PrototypeGenTool) Name() string { return "generate_prototype" }

func (t *PrototypeGenTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "generate_prototype",
			Description: "生成 HTML 原型页面。根据指定的平台和页面描述，生成包含 Bootstrap 5 + Alpine.js 的完整 HTML 原型代码。每个页面都是独立可运行的 HTML 文件。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"platform": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"web", "mobile", "desktop"},
						"description": "目标平台：web（Web 端，1920x1080）、mobile（移动端，393x852）、desktop（桌面端，1920x1080）",
					},
					"pages": map[string]interface{}{
						"type":        "array",
						"description": "页面列表",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"filename": map[string]interface{}{
									"type":        "string",
									"description": "文件名（如 index.html、login.html）",
								},
								"title": map[string]interface{}{
									"type":        "string",
									"description": "页面标题",
								},
								"description": map[string]interface{}{
									"type":        "string",
									"description": "页面功能描述",
								},
								"body_html": map[string]interface{}{
									"type":        "string",
									"description": "页面 body 内的 HTML 内容（使用 Bootstrap 5 组件）",
								},
							},
							"required": []string{"filename", "title", "body_html"},
						},
					},
				},
				"required": []string{"platform", "pages"},
			},
		},
	}
}

type prototypePage struct {
	Filename    string `json:"filename"`
	Title       string `json:"title"`
	Description string `json:"description"`
	BodyHTML    string `json:"body_html"`
}

type prototypeGenArgs struct {
	Platform string          `json:"platform"`
	Pages    []prototypePage `json:"pages"`
}

var viewportMeta = map[string]string{
	"mobile":  `<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no" />`,
	"web":     `<meta name="viewport" content="width=device-width, initial-scale=1.0" />`,
	"desktop": `<meta name="viewport" content="width=device-width, initial-scale=1.0" />`,
}

func (t *PrototypeGenTool) Execute(_ context.Context, arguments string) (string, error) {
	var args prototypeGenArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Platform == "" {
		return "", fmt.Errorf("platform is required")
	}
	if len(args.Pages) == 0 {
		return "", fmt.Errorf("at least one page is required")
	}

	viewport, ok := viewportMeta[args.Platform]
	if !ok {
		viewport = viewportMeta["web"]
	}

	platformLabel := map[string]string{
		"mobile":  "移动端",
		"web":     "Web 端",
		"desktop": "桌面端",
	}[args.Platform]

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("已生成 %s 原型（%d 个页面）：\n\n", platformLabel, len(args.Pages)))

	for _, page := range args.Pages {
		html := buildPrototypeHTML(page, viewport)
		sb.WriteString(fmt.Sprintf("### %s（%s）\n", page.Title, page.Filename))
		if page.Description != "" {
			sb.WriteString(page.Description + "\n")
		}
		sb.WriteString("\n```html\n")
		sb.WriteString(html)
		sb.WriteString("\n```\n\n")
	}

	sb.WriteString("将以上代码分别保存为对应的 .html 文件，在浏览器中打开即可查看原型。")
	return sb.String(), nil
}

func buildPrototypeHTML(page prototypePage, viewport string) string {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n")
	sb.WriteString("<html lang=\"zh\">\n")
	sb.WriteString("<head>\n")
	sb.WriteString("  <meta charset=\"UTF-8\" />\n")
	sb.WriteString("  " + viewport + "\n")
	sb.WriteString(fmt.Sprintf("  <title>%s</title>\n", page.Title))
	sb.WriteString("  <link href=\"https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css\" rel=\"stylesheet\" />\n")
	sb.WriteString("</head>\n")
	sb.WriteString("<body>\n")
	sb.WriteString(page.BodyHTML)
	sb.WriteString("\n")
	sb.WriteString("  <script defer src=\"https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js\"></script>\n")
	sb.WriteString("  <script src=\"https://unpkg.com/lucide@latest\"></script>\n")
	sb.WriteString("  <script>\n")
	sb.WriteString("    document.addEventListener('DOMContentLoaded', () => {\n")
	sb.WriteString("      if (typeof lucide !== 'undefined') lucide.createIcons();\n")
	sb.WriteString("    });\n")
	sb.WriteString("  </script>\n")
	sb.WriteString("</body>\n")
	sb.WriteString("</html>")
	return sb.String()
}
