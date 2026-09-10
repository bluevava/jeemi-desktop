package chainproxy

import "strings"

// NameMatcher shares subscription/selectorSearch.ts semantics: OR bucket,
// followed by all required terms, followed by all exclusions. No regex/eval.
type NameMatcher struct{ any, all, exclude []string }

func ParseNameFilter(query string) NameMatcher {
	var result NameMatcher
	op, start := '|', 0
	appendTerm := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			return
		}
		switch op {
		case '&':
			result.all = append(result.all, term)
		case '!':
			result.exclude = append(result.exclude, term)
		default:
			result.any = append(result.any, term)
		}
	}
	for index, char := range query {
		if char != '|' && char != '&' && char != '!' {
			continue
		}
		appendTerm(query[start:index])
		op = char
		start = index + 1
	}
	appendTerm(query[start:])
	return result
}

func (m NameMatcher) Empty() bool { return len(m.any)+len(m.all)+len(m.exclude) == 0 }
func (m NameMatcher) Match(name string) bool {
	name = strings.ToLower(name)
	any := len(m.any) == 0
	for _, term := range m.any {
		any = any || strings.Contains(name, term)
	}
	if !any {
		return false
	}
	for _, term := range m.all {
		if !strings.Contains(name, term) {
			return false
		}
	}
	for _, term := range m.exclude {
		if strings.Contains(name, term) {
			return false
		}
	}
	return true
}
