// Package chat turns a message plus its context into a prompt, runs it on a
// runtime, and persists the transcript.
package chat

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ai-skope/aiss/internal/files"
	"github.com/ai-skope/aiss/internal/store"
)

// Packed is a prompt assembled from a question and its context.
type Packed struct {
	Prompt    string
	FilePaths []string // resolved paths, used to choose the working directory
	Bytes     int
}

// priority orders context in the prompt: the most specific thing the user
// aimed at comes first, the whole page last.
func priority(t string) int {
	switch t {
	case store.ContextFile:
		return 0
	case store.ContextElement:
		return 1
	case store.ContextText:
		return 2
	default:
		return 3
	}
}

// Pack builds the prompt. Every piece of context is included, but the whole
// thing is held under budget: each item gets a fair share of what is left, and
// anything cut says so, so the model is never silently given half a table.
func Pack(guard *files.Guard, question string, page *store.ContextItem, items []store.ContextItem, budget int) Packed {
	if budget <= 0 {
		budget = 24000
	}
	ordered := append([]store.ContextItem{}, items...)
	if page != nil {
		ordered = append(ordered, *page)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return priority(ordered[i].Type) < priority(ordered[j].Type)
	})

	var sb strings.Builder
	sb.WriteString("You are AI Skope, answering about what the user is looking at in their browser.\n")
	sb.WriteString("Ground every claim in the material below. If it does not say, say so.\n")

	remaining := budget - len(question) - sb.Len()
	if remaining < 0 {
		remaining = 0
	}
	var paths []string

	for i, it := range ordered {
		left := len(ordered) - i
		share := remaining / max(1, left)
		if share < 400 {
			share = min(remaining, 400)
		}
		block, path := renderItem(guard, it, share)
		if path != "" {
			paths = append(paths, path)
		}
		if block == "" {
			continue
		}
		sb.WriteString("\n")
		sb.WriteString(block)
		remaining -= len(block)
		if remaining < 0 {
			remaining = 0
		}
	}

	sb.WriteString("\n## Question\n")
	sb.WriteString(question)
	sb.WriteString("\n")
	return Packed{Prompt: sb.String(), FilePaths: paths, Bytes: sb.Len()}
}

// renderItem renders one context item within its byte share, returning the
// block and (for files) the resolved path.
func renderItem(guard *files.Guard, it store.ContextItem, share int) (string, string) {
	var sb strings.Builder
	switch it.Type {
	case store.ContextFile:
		if it.Path == "" {
			return "", ""
		}
		content, err := guard.Read(it.Path)
		if err != nil {
			// The picker only offers readable files, so this means the file
			// changed or access was revoked. Say so rather than pretend.
			fmt.Fprintf(&sb, "## Local file: %s\n(could not be read: %v)\n", files.Tilde(it.Path), err)
			return sb.String(), ""
		}
		fmt.Fprintf(&sb, "## Local file: %s\n", content.Display)
		if content.Title != "" {
			fmt.Fprintf(&sb, "Title: %s\n", content.Title)
		}
		body, cut := clip(content.Text, share)
		fmt.Fprintf(&sb, "```\n%s\n```\n", body)
		if cut || content.Truncated {
			fmt.Fprintf(&sb, "(truncated — the full file is at %s and you can open it)\n", content.Path)
		}
		return sb.String(), content.Path

	case store.ContextElement:
		fmt.Fprintf(&sb, "## Picked element: %s\n", orDash(it.Selector))
		if len(it.Rect) == 2 {
			fmt.Fprintf(&sb, "Size: %d × %d\n", it.Rect[0], it.Rect[1])
		}
		if it.Text != "" {
			body, cut := clip(it.Text, share*2/3)
			fmt.Fprintf(&sb, "Text:\n%s\n", body)
			if cut {
				sb.WriteString("(truncated)\n")
			}
		}
		if it.HTML != "" {
			body, cut := clip(it.HTML, share/3)
			fmt.Fprintf(&sb, "HTML:\n```html\n%s\n```\n", body)
			if cut {
				sb.WriteString("(truncated)\n")
			}
		}
		return sb.String(), ""

	case store.ContextText:
		body, cut := clip(it.Quote, share)
		sb.WriteString("## Selected text\n")
		fmt.Fprintf(&sb, "> %s\n", strings.ReplaceAll(body, "\n", "\n> "))
		if cut {
			sb.WriteString("(truncated)\n")
		}
		if it.Before != "" || it.After != "" {
			b, _ := clip(it.Before, 200)
			a, _ := clip(it.After, 200)
			fmt.Fprintf(&sb, "Surrounding text: …%s [selection] %s…\n", b, a)
		}
		return sb.String(), ""

	case store.ContextPage:
		sb.WriteString("## Page\n")
		if it.URL != "" {
			fmt.Fprintf(&sb, "URL: %s\n", it.URL)
		}
		if it.Title != "" {
			fmt.Fprintf(&sb, "Title: %s\n", it.Title)
		}
		if it.Text != "" {
			body, cut := clip(it.Text, share)
			fmt.Fprintf(&sb, "Content:\n%s\n", body)
			if cut {
				sb.WriteString("(truncated — this is the top of the page)\n")
			}
		}
		return sb.String(), ""
	}
	return "", ""
}

// clip truncates on a rune boundary and reports whether it cut anything.
func clip(s string, limit int) (string, bool) {
	if limit <= 0 {
		return "", s != ""
	}
	if len(s) <= limit {
		return s, false
	}
	cut := s[:limit]
	for len(cut) > 0 && !isRuneStart(cut[len(cut)-1]) {
		cut = cut[:len(cut)-1]
	}
	return strings.TrimSpace(cut), true
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

func orDash(s string) string {
	if s == "" {
		return "(unnamed)"
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
