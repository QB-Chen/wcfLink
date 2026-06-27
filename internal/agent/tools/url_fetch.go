package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QB-Chen/wcfLink/internal/llm"
)

var (
	reComment   = regexp.MustCompile(`<!--[\s\S]*?-->`)
	reHTMLTag   = regexp.MustCompile(`<[^>]+>`)
	reNumEntity = regexp.MustCompile(`&#(\d+);`)
	reBlockTags = regexp.MustCompile(`(?i)</?(?:div|p|br|h[1-6]|li|tr|td|th|blockquote|pre|article|section)[^>]*>`)
	reScript    = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle     = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reNoscript  = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	reNav       = regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
	reFooter    = regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	reHeader    = regexp.MustCompile(`(?is)<header[^>]*>.*?</header>`)
)

type URLFetchTool struct {
	httpClient       *http.Client
	maxContentLength int
}

func NewURLFetchTool(maxContentLength int) *URLFetchTool {
	if maxContentLength <= 0 {
		maxContentLength = 8000
	}
	return &URLFetchTool{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		maxContentLength: maxContentLength,
	}
}

func (t *URLFetchTool) Name() string { return "url_content_fetch" }

func (t *URLFetchTool) Definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Type: "function",
		Function: llm.FunctionDefinition{
			Name:        "url_content_fetch",
			Description: "获取指定 URL 的网页内容，转换为纯文本格式返回。适用于深入阅读搜索结果中的详细内容。",
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

type fetchArgs struct {
	URL string `json:"url"`
}

func (t *URLFetchTool) Execute(ctx context.Context, arguments string) (string, error) {
	var args fetchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if args.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return fmt.Sprintf("无法访问 URL: %v", err), nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Sprintf("请求失败: %v", err), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Sprintf("请求返回状态码 %d", resp.StatusCode), nil
	}

	limited := io.LimitReader(resp.Body, 2*1024*1024)
	body, err := io.ReadAll(limited)
	if err != nil {
		return fmt.Sprintf("读取内容失败: %v", err), nil
	}

	text := htmlToText(string(body))
	text = collapseWhitespace(text)

	if utf8.RuneCountInString(text) > t.maxContentLength {
		runes := []rune(text)
		text = string(runes[:t.maxContentLength]) + "\n\n[内容已截断]"
	}

	if strings.TrimSpace(text) == "" {
		return "页面内容为空或无法提取文本内容。", nil
	}

	return fmt.Sprintf("URL: %s\n\n%s", args.URL, text), nil
}

func htmlToText(html string) string {
	html = reScript.ReplaceAllString(html, "")
	html = reStyle.ReplaceAllString(html, "")
	html = reNoscript.ReplaceAllString(html, "")
	html = reNav.ReplaceAllString(html, "")
	html = reFooter.ReplaceAllString(html, "")
	html = reHeader.ReplaceAllString(html, "")

	html = reComment.ReplaceAllString(html, "")
	html = reBlockTags.ReplaceAllString(html, "\n")
	html = reHTMLTag.ReplaceAllString(html, "")

	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = reNumEntity.ReplaceAllString(html, " ")

	return html
}

func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	prevEmpty := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !prevEmpty {
				result = append(result, "")
				prevEmpty = true
			}
			continue
		}
		prevEmpty = false
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}
