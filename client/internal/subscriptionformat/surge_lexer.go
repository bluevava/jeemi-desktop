package subscriptionformat

import "strings"

func stripSurgeComment(line string) string {
	var quote rune
	escaped := false
	for index, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 && r == '\\' {
			escaped = true
			continue
		}
		if r == '\'' || r == '"' {
			if quote == 0 {
				quote = r
			} else if quote == r {
				quote = 0
			}
			continue
		}
		if quote == 0 && (r == '#' || r == ';') && index > 0 && (line[index-1] == ' ' || line[index-1] == '\t') {
			return strings.TrimSpace(line[:index])
		}
	}
	return line
}

// Delimiters inside quoted values are data. Preserve quotes until the caller
// decodes the individual value, so quoted credential whitespace is retained.
func splitSurge(value string, delimiter rune) ([]string, bool) {
	parts := []string{}
	start := 0
	var quote rune
	escaped := false
	for index, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' || r == '\'' {
			if quote == 0 {
				quote = r
			} else if quote == r {
				quote = 0
			}
			continue
		}
		if r == delimiter && quote == 0 {
			parts = append(parts, strings.TrimSpace(value[start:index]))
			start = index + 1
		}
	}
	if quote != 0 || escaped {
		return nil, false
	}
	return append(parts, strings.TrimSpace(value[start:])), true
}

func surgeAssignment(line string) (string, string, bool) {
	var quote rune
	escaped := false
	for index, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' || r == '\'' {
			if quote == 0 {
				quote = r
			} else if quote == r {
				quote = 0
			}
			continue
		}
		if r == '=' && quote == 0 {
			return strings.TrimSpace(line[:index]), strings.TrimSpace(line[index+1:]), true
		}
	}
	return "", "", false
}

func surgeValue(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", true
	}
	quote := value[0]
	if quote != '"' && quote != '\'' {
		return value, !strings.ContainsAny(value, "\"'")
	}
	if len(value) < 2 || value[len(value)-1] != quote {
		return "", false
	}
	var b strings.Builder
	for i := 1; i < len(value)-1; i++ {
		c := value[i]
		if c == quote {
			return "", false
		}
		if c == '\\' && i+1 < len(value)-1 && (value[i+1] == quote || value[i+1] == '\\') {
			i++
			c = value[i]
		}
		b.WriteByte(c)
	}
	return b.String(), true
}
