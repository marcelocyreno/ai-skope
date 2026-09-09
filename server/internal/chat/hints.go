package chat

import (
	"strings"
	"unicode"

	"github.com/ai-skope/aiss/internal/store"
)

// How many index matches a prompt carries: as paths for an agent that can
// open them, or inlined for one that cannot. Inlined files share the context
// budget, so fewer of them fit.
const (
	maxHints       = 6
	maxInlineHints = 3
)

// stopWords are dropped before a question is matched against the index. They
// are in every question and in every file, so they only add noise; the words
// that stay are the ones that can single a file out.
var stopWords = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`
		a an the and or but if is are was were be been being am do does did done
		have has had having this that these those it its of in on at to for from
		by with about into over under as than then so not no yes what which who
		whom whose when where why how can could should would will shall may
		might must i me my mine we us our you your he she they them their his
		her there here any all some one two also just only very more most much
		many tell explain show please mean means like say says said page file
		files code thing things use using used want need know make get look
		find give see let out up down new old way`) {
		stopWords[w] = true
	}
}

// queryTerms turns free text into the words worth matching: lower-cased,
// split on anything that is not a letter or digit (the way the index
// tokenises), with stop words, very short tokens and repeats removed.
func queryTerms(parts ...string) []string {
	const limit = 12
	seen := map[string]bool{}
	var out []string
	for _, p := range parts {
		fields := strings.FieldsFunc(strings.ToLower(p), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		for _, f := range fields {
			if len([]rune(f)) < 3 || stopWords[f] || seen[f] {
				continue
			}
			seen[f] = true
			out = append(out, f)
			if len(out) == limit {
				return out
			}
		}
	}
	return out
}

// suggest asks the index which files a question is most likely about. Files
// already in the context are left out: the model has them, and repeating them
// would only push a real find off the list.
func suggest(db *store.DB, question, pageTitle string, exclude []string, limit int) []store.File {
	terms := queryTerms(question, pageTitle)
	if len(terms) == 0 {
		return nil
	}
	skip := map[string]bool{}
	for _, p := range exclude {
		if p != "" {
			skip[p] = true
		}
	}
	hits, err := db.SuggestFiles(terms, limit+len(skip))
	if err != nil {
		return nil
	}
	var out []store.File
	for _, h := range hits {
		if skip[h.Path] {
			continue
		}
		out = append(out, h)
		if len(out) == limit {
			break
		}
	}
	return out
}
