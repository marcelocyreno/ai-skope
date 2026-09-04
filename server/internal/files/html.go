package files

import (
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// skipTags never contribute readable text.
var skipTags = map[atom.Atom]bool{
	atom.Script: true, atom.Style: true, atom.Noscript: true,
	atom.Nav: true, atom.Svg: true, atom.Iframe: true, atom.Template: true,
}

// blockTags force a line break, so the extracted text keeps its shape.
var blockTags = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Section: true, atom.Article: true,
	atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true,
	atom.Li: true, atom.Br: true, atom.Tr: true, atom.Table: true, atom.Pre: true,
	atom.Blockquote: true, atom.Header: true, atom.Footer: true, atom.Main: true,
	atom.Ul: true, atom.Ol: true, atom.Dl: true, atom.Dt: true, atom.Dd: true,
}

// HTMLToText renders an HTML document as readable plain text: headings,
// paragraphs, lists and table rows are kept, scripts and navigation dropped.
// Headings are prefixed with Markdown hashes so structure survives.
func HTMLToText(r io.Reader) (string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skipTags[n.DataAtom] {
			return
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				sb.WriteString(t)
				sb.WriteByte(' ')
			}
			return
		}
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.H1:
				sb.WriteString("\n# ")
			case atom.H2:
				sb.WriteString("\n## ")
			case atom.H3:
				sb.WriteString("\n### ")
			case atom.H4, atom.H5, atom.H6:
				sb.WriteString("\n#### ")
			case atom.Li:
				sb.WriteString("\n- ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && blockTags[n.DataAtom] {
			sb.WriteByte('\n')
		}
	}
	walk(doc)
	return squeezeBlankLines(sb.String()), nil
}

// Title returns the document's <title>, when it has one.
func Title(doc string) string {
	n, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return ""
	}
	var out string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if out != "" {
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Title && n.FirstChild != nil {
			out = strings.TrimSpace(n.FirstChild.Data)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func squeezeBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		l = strings.TrimRight(strings.TrimSpace(l), " ")
		if l == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
