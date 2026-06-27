package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QB-Chen/wcfLink/internal/llm"
)

type WebSearchTool struct {
	httpClient *http.Client
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "web_search",
			Description: "搜索互联网获取最新信息。返回搜索结果列表（标题、URL、摘要）。可以搜索通用内容、小红书、知乎、微博等平台。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "搜索关键词",
					},
					"search_on": map[string]interface{}{
						"type":        "string",
						"description": "搜索平台：general（通用搜索引擎，默认）、xiaohongshu（小红书）、zhihu（知乎）、weibo（微博）",
						"enum":        []string{"general", "xiaohongshu", "zhihu", "weibo"},
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "最大返回结果数（1-20，默认 10）",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

type searchArgs struct {
	Query      string `json:"query"`
	SearchOn   string `json:"search_on"`
	MaxResults int    `json:"max_results"`
}

type searchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

func (t *WebSearchTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args searchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	if args.MaxResults <= 0 || args.MaxResults > 20 {
		args.MaxResults = 10
	}
	if args.SearchOn == "" {
		args.SearchOn = "general"
	}

	query := args.Query
	switch args.SearchOn {
	case "xiaohongshu":
		query = "site:xiaohongshu.com " + query
	case "zhihu":
		query = "site:zhihu.com " + query
	case "weibo":
		query = "site:weibo.com " + query
	}

	results, err := t.duckduckgoSearch(ctx, query, args.MaxResults)
	if err != nil {
		results, err = t.bingHTMLSearch(ctx, query, args.MaxResults)
		if err != nil {
			return fmt.Sprintf("搜索失败: %v", err), nil
		}
	}

	if len(results) == 0 {
		return "未找到相关结果。请尝试更换关键词。", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索 \"%s\" 找到 %d 条结果：\n\n", args.Query, len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n   %s\n   %s\n\n", i+1, r.Title, r.URL, r.Description))
	}
	return sb.String(), nil
}

func (t *WebSearchTool) duckduckgoSearch(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	u := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseDuckDuckGoHTML(string(body), maxResults), nil
}

func parseDuckDuckGoHTML(html string, maxResults int) []searchResult {
	var results []searchResult
	parts := strings.Split(html, "class=\"result__a\"")
	for i := 1; i < len(parts) && len(results) < maxResults; i++ {
		part := parts[i]

		href := extractAttr(part, "href=\"")
		title := extractTextContent(part)
		desc := ""
		if descIdx := strings.Index(part, "class=\"result__snippet\""); descIdx >= 0 {
			desc = extractTextContent(part[descIdx:])
		}

		if href != "" && title != "" {
			if strings.HasPrefix(href, "//duckduckgo.com/l/?uddg=") {
				if decoded, err := url.QueryUnescape(strings.TrimPrefix(href, "//duckduckgo.com/l/?uddg=")); err == nil {
					if ampIdx := strings.Index(decoded, "&"); ampIdx > 0 {
						decoded = decoded[:ampIdx]
					}
					href = decoded
				}
			}
			results = append(results, searchResult{
				Title:       cleanHTML(title),
				URL:         href,
				Description: cleanHTML(desc),
			})
		}
	}
	return results
}

func (t *WebSearchTool) bingHTMLSearch(ctx context.Context, query string, maxResults int) ([]searchResult, error) {
	u := fmt.Sprintf("https://www.bing.com/search?q=%s&count=%d", url.QueryEscape(query), maxResults)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseBingHTML(string(body), maxResults), nil
}

func parseBingHTML(html string, maxResults int) []searchResult {
	var results []searchResult
	parts := strings.Split(html, "<li class=\"b_algo\"")
	for i := 1; i < len(parts) && len(results) < maxResults; i++ {
		part := parts[i]

		href := ""
		if hIdx := strings.Index(part, "href=\""); hIdx >= 0 {
			href = extractAttr(part[hIdx:], "href=\"")
		}
		title := ""
		if aIdx := strings.Index(part, "<a "); aIdx >= 0 {
			title = extractTextContent(part[aIdx:])
		}
		desc := ""
		if pIdx := strings.Index(part, "<p"); pIdx >= 0 {
			desc = extractTextContent(part[pIdx:])
		}

		if href != "" && title != "" {
			results = append(results, searchResult{
				Title:       cleanHTML(title),
				URL:         href,
				Description: cleanHTML(desc),
			})
		}
	}
	return results
}

func extractAttr(s, prefix string) string {
	idx := strings.Index(s, prefix)
	if idx < 0 {
		return ""
	}
	start := idx + len(prefix)
	end := strings.Index(s[start:], "\"")
	if end < 0 {
		return ""
	}
	return s[start : start+end]
}

func extractTextContent(s string) string {
	end := strings.Index(s, "</a>")
	if end < 0 {
		end = strings.Index(s, "</p>")
	}
	if end < 0 {
		end = strings.Index(s, "</span>")
	}
	if end < 0 {
		if len(s) > 500 {
			end = 500
		} else {
			end = len(s)
		}
	}
	text := s[:end]
	return cleanHTML(text)
}

func cleanHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	out := strings.TrimSpace(result.String())
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", "\"")
	out = strings.ReplaceAll(out, "&#39;", "'")
	return out
}
