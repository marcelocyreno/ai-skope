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

// Input is everything a prompt is built from.
type Input struct {
	Question string
	Page     *store.ContextItem
	Items    []store.ContextItem
	Budget   int

	// Folders is the allow-list, and WorkDir the directory the agent will
	// run in. Both are named in the prompt so the agent knows it may look
	// there, and where it already is.
	Folders []store.Folder
	WorkDir string

	// Hits are the index's best matches for the question, best first. An
	// agent that can open files (see runtime.Spec.ReadsFiles) is given them
	// as paths to start from; one that cannot has their content inlined.
	Hits       []store.File
	InlineHits bool
}

// Packed is a prompt assembled from a question and its context.
type Packed struct {
	Prompt    string
	FilePaths []string // resolved paths of the attached files
	Bytes     int
}

// contextHit is an index match rendered as inlined content. It is internal
// to the packer: hits are found per turn, never stored on a message.
const contextHit = "index-hit"

// priority orders context in the prompt: the most specific thing the user
// aimed at comes first, the whole page after, and what the server found on
// its own last.
func priority(t string) int {
	switch t {
	case store.ContextFile:
		return 0
	case store.ContextElement:
		return 1
	case store.ContextText:
		return 2
	case store.ContextPage:
		return 3
	default:
		return 4
	}
}

// Pack builds the prompt. Every piece of context is included, but the whole
// thing is held under budget: each item gets a fair share of what is left, and
// anything cut says so, so the model is never silently given half a table.
func Pack(guard *files.Guard, in Input) Packed {
	budget := in.Budget
	if budget <= 0 {
		budget = 24000
	}
	ordered := append([]store.ContextItem{}, in.Items...)
	if in.Page != nil {
		ordered = append(ordered, *in.Page)
	}
	if in.InlineHits {
		for i, h := range in.Hits {
			if i == maxInlineHints {
				break
			}
			ordered = append(ordered, store.ContextItem{Type: contextHit, Path: h.Path})
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return priority(ordered[i].Type) < priority(ordered[j].Type)
	})

	var sb strings.Builder
	sb.WriteString("You are AI Skope, answering questions about what the user is looking at in their browser.\n")
	sb.WriteString("Answer the question directly and completely; do not propose a plan or ask for approval. ")
	sb.WriteString("Ground every claim in the material below or in files you read, and say plainly what you could not find.\n")
	sb.WriteString("\n")
	sb.WriteString(renderFolders(in))
	if !in.InlineHits {
		sb.WriteString(renderHints(in.Hits))
	}

	remaining := budget - len(in.Question) - sb.Len()
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
		if path != "" && it.Type == store.ContextFile {
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
	sb.WriteString(in.Question)
	sb.WriteString("\n")
	return Packed{Prompt: sb.String(), FilePaths: paths, Bytes: sb.Len()}
}

// renderFolders tells the agent what it may read on disk and where it runs.
// Without this an agent has no reason to look past the material it was
// handed, and answers "the page does not say" about a repository it is
// sitting in.
func renderFolders(in Input) string {
	var sb strings.Builder
	sb.WriteString("## Local folders\n")
	if len(in.Folders) == 0 {
		sb.WriteString("The user has not allowed any folder on their computer yet, so answer from the material below.\n")
		return sb.String()
	}
	sb.WriteString("The user allows you to read these folders on their computer, and nothing else on disk:\n")
	for _, f := range in.Folders {
		fmt.Fprintf(&sb, "- %s", files.Tilde(f.Path))
		if f.FileCount > 0 {
			fmt.Fprintf(&sb, " (%d files)", f.FileCount)
		}
		sb.WriteString("\n")
	}
	if in.InlineHits {
		sb.WriteString("You have no file tools in this session; the files from those folders that best match the question are included below.\n")
		return sb.String()
	}
	if in.WorkDir != "" {
		fmt.Fprintf(&sb, "Your working directory is %s.\n", files.Tilde(in.WorkDir))
	}
	sb.WriteString("Search, list and read these folders with your tools whenever the question concerns them or the material below is not enough. Never modify anything.\n")
	return sb.String()
}

// renderHints lists the index's best matches as places to start.
func renderHints(hits []store.File) string {
	if len(hits) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n## Possibly relevant files\n")
	sb.WriteString("From the local index, best match first. Open the ones that matter.\n")
	for i, h := range hits {
		if i == maxHints {
			break
		}
		fmt.Fprintf(&sb, "- %s", files.Tilde(h.Path))
		if snip := strings.TrimSpace(strings.ReplaceAll(h.Snippet, "\n", " ")); snip != "" {
			snip, _ = clip(snip, 160)
			fmt.Fprintf(&sb, " — %s", snip)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// renderItem renders one context item within its byte share, returning the
// block and (for files) the resolved path.
func renderItem(guard *files.Guard, it store.ContextItem, share int) (string, string) {
	var sb strings.Builder
	switch it.Type {
	case store.ContextFile, contextHit:
		if it.Path == "" {
			return "", ""
		}
		heading := "Local file"
		if it.Type == contextHit {
			heading = "Possibly relevant local file"
		}
		content, err := guard.Read(it.Path)
		if err != nil {
			if it.Type == contextHit {
				return "", "" // the server's own find; nothing to report
			}
			// The picker only offers readable files, so this means the file
			// changed or access was revoked. Say so rather than pretend.
			fmt.Fprintf(&sb, "## %s: %s\n(could not be read: %v)\n", heading, files.Tilde(it.Path), err)
			return sb.String(), ""
		}
		fmt.Fprintf(&sb, "## %s: %s\n", heading, content.Display)
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
		if it.Path != "" {
			// A page served from an allowed folder: the agent can open the
			// file itself, and the folder around it is the project.
			fmt.Fprintf(&sb, "This page is the local file %s; open it for the full content.\n", files.Tilde(it.Path))
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
