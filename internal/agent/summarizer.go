package agent

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/QB-Chen/wcfLink/internal/llm"
)

const (
	maxContextChars      = 24000
	reservedChars        = 4000
	systemPromptReserved = 4000
	summaryPrompt        = `请将以下对话历史压缩为一段简洁的摘要。保留关键信息：用户的需求、重要的搜索结果、已做出的决定和待完成的事项。摘要使用中文。

对话历史：
%s

请输出摘要：`
)

func estimateMessageChars(msgs []llm.Message) int {
	total := 0
	for _, m := range msgs {
		total += utf8.RuneCountInString(m.Content)
		for _, tc := range m.ToolCalls {
			total += utf8.RuneCountInString(tc.Function.Arguments)
		}
	}
	return total
}

func needsSummarization(systemPrompt string, history []llm.Message) bool {
	promptChars := utf8.RuneCountInString(systemPrompt)
	historyChars := estimateMessageChars(history)
	return promptChars+historyChars > maxContextChars-reservedChars
}

func summarizeHistory(ctx context.Context, client *llm.Client, history []llm.Message, temperature *float64, maxTokens int) (string, int, error) {
	if len(history) < 4 {
		return "", 0, nil
	}

	// Find a clean cut boundary at a RoleUser message.
	// Search backward from midpoint first, then forward, to always land on a user message.
	cutPoint := -1
	mid := len(history) / 2
	for i := mid; i > 0; i-- {
		if history[i].Role == llm.RoleUser {
			cutPoint = i
			break
		}
	}
	if cutPoint < 0 {
		for i := mid + 1; i < len(history); i++ {
			if history[i].Role == llm.RoleUser {
				cutPoint = i
				break
			}
		}
	}
	if cutPoint <= 0 || cutPoint >= len(history) {
		return "", 0, nil
	}

	oldMessages := history[:cutPoint]
	var dialogText string
	for _, m := range oldMessages {
		switch m.Role {
		case llm.RoleUser:
			dialogText += fmt.Sprintf("用户: %s\n", m.Content)
		case llm.RoleAssistant:
			if m.Content != "" {
				dialogText += fmt.Sprintf("助手: %s\n", m.Content)
			}
		case llm.RoleTool:
			if utf8.RuneCountInString(m.Content) > 200 {
				dialogText += fmt.Sprintf("工具结果: %s...\n", string([]rune(m.Content)[:200]))
			} else {
				dialogText += fmt.Sprintf("工具结果: %s\n", m.Content)
			}
		}
	}

	prompt := fmt.Sprintf(summaryPrompt, dialogText)
	resp, err := client.Chat(ctx, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", 0, fmt.Errorf("summarize call failed: %w", err)
	}

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", 0, fmt.Errorf("empty summary response")
	}

	return resp.Choices[0].Message.Content, cutPoint, nil
}

func rebuildCompacted(summary string, cutPoint int, history []llm.Message) []llm.Message {
	if cutPoint >= len(history) {
		cutPoint = 0
	}
	compacted := make([]llm.Message, 0, len(history)-cutPoint+2)
	compacted = append(compacted, llm.Message{
		Role:    llm.RoleUser,
		Content: fmt.Sprintf("[之前的对话摘要]\n%s", summary),
	})
	compacted = append(compacted, llm.Message{
		Role:    llm.RoleAssistant,
		Content: "好的，我已了解之前的对话内容。请继续。",
	})
	compacted = append(compacted, history[cutPoint:]...)
	return compacted
}
