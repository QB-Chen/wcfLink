package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QB-Chen/wcfLink/internal/llm"
)

type SocialSearchTool struct {
	httpClient *http.Client
}

func NewSocialSearchTool() *SocialSearchTool {
	return &SocialSearchTool{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *SocialSearchTool) Name() string { return "social_search" }

func (t *SocialSearchTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "social_search",
			Description: "社交平台专项搜索。针对小红书、知乎、微博、Reddit 等平台进行定向搜索，返回该平台上的相关内容。适合收集用户评价、社区讨论、真实体验等。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"platform": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"xiaohongshu", "zhihu", "weibo", "reddit"},
						"description": "目标平台：xiaohongshu（小红书）、zhihu（知乎）、weibo（微博）、reddit（Reddit）",
					},
					"keyword": map[string]interface{}{
						"type":        "string",
						"description": "搜索关键词",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "返回结果数量（1-20，默认 10）",
					},
				},
				"required": []string{"platform", "keyword"},
			},
		},
	}
}

type socialSearchArgs struct {
	Platform string `json:"platform"`
	Keyword  string `json:"keyword"`
	Limit    int    `json:"limit"`
}

var platformNames = map[string]string{
	"xiaohongshu": "小红书",
	"zhihu":       "知乎",
	"weibo":       "微博",
	"reddit":      "Reddit",
}

var platformDomains = map[string]string{
	"xiaohongshu": "xiaohongshu.com",
	"zhihu":       "zhihu.com",
	"weibo":       "weibo.com",
	"reddit":      "reddit.com",
}

func (t *SocialSearchTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args socialSearchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Keyword == "" {
		return "", fmt.Errorf("keyword is required")
	}
	if args.Platform == "" {
		return "", fmt.Errorf("platform is required")
	}
	if args.Limit <= 0 || args.Limit > 20 {
		args.Limit = 10
	}

	domain, ok := platformDomains[args.Platform]
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s", args.Platform)
	}

	query := fmt.Sprintf("site:%s %s", domain, args.Keyword)

	webSearch := &WebSearchTool{httpClient: t.httpClient}

	results, err := webSearch.duckduckgoSearch(ctx, query, args.Limit)
	if err != nil {
		results, err = webSearch.bingHTMLSearch(ctx, query, args.Limit)
		if err != nil {
			return fmt.Sprintf("%s 搜索失败: %v", platformNames[args.Platform], err), nil
		}
	}

	if len(results) == 0 {
		return fmt.Sprintf("在%s上未找到「%s」的相关内容。请尝试更换关键词。", platformNames[args.Platform], args.Keyword), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("在%s上搜索「%s」找到 %d 条结果：\n\n", platformNames[args.Platform], args.Keyword, len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description))
	}
	return sb.String(), nil
}
