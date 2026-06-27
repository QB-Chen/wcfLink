package agent

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxMessageLength = 1800

func SplitMessage(text string) []string {
	if utf8.RuneCountInString(text) <= maxMessageLength {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n\n")
	var segments []string
	var current strings.Builder

	for _, para := range paragraphs {
		paraLen := utf8.RuneCountInString(para)
		currentLen := utf8.RuneCountInString(current.String())

		if currentLen > 0 && currentLen+paraLen+2 > maxMessageLength {
			segments = append(segments, strings.TrimSpace(current.String()))
			current.Reset()
		}

		if paraLen > maxMessageLength {
			if current.Len() > 0 {
				segments = append(segments, strings.TrimSpace(current.String()))
				current.Reset()
			}
			segments = append(segments, splitLongParagraph(para)...)
			continue
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
	}

	if current.Len() > 0 {
		segments = append(segments, strings.TrimSpace(current.String()))
	}

	if len(segments) <= 1 {
		return segments
	}

	total := len(segments)
	for i := range segments {
		segments[i] = fmt.Sprintf("[%d/%d]\n%s", i+1, total, segments[i])
	}
	return segments
}

func splitLongParagraph(para string) []string {
	lines := strings.Split(para, "\n")
	var segments []string
	var current strings.Builder

	for _, line := range lines {
		lineLen := utf8.RuneCountInString(line)
		currentLen := utf8.RuneCountInString(current.String())

		if currentLen > 0 && currentLen+lineLen+1 > maxMessageLength {
			segments = append(segments, strings.TrimSpace(current.String()))
			current.Reset()
		}

		if lineLen > maxMessageLength {
			if current.Len() > 0 {
				segments = append(segments, strings.TrimSpace(current.String()))
				current.Reset()
			}
			runes := []rune(line)
			for len(runes) > 0 {
				end := maxMessageLength
				if end > len(runes) {
					end = len(runes)
				}
				segments = append(segments, string(runes[:end]))
				runes = runes[end:]
			}
			continue
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		segments = append(segments, strings.TrimSpace(current.String()))
	}
	return segments
}
