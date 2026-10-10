// Package sqlsafe checks SQL text before it is sent to an isolated database.
// The checks are a denylist, not a sandbox. Isolation is the real control.
package sqlsafe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type pattern struct {
	name string
	re   *regexp.Regexp
}

var denied = []pattern{
	{"pg_read_file", regexp.MustCompile(`(?i)\bpg_read_file\s*\(`)},
	{"pg_read_binary_file", regexp.MustCompile(`(?i)\bpg_read_binary_file\s*\(`)},
	{"pg_ls_dir", regexp.MustCompile(`(?i)\bpg_ls_dir\s*\(`)},
	{"pg_stat_file", regexp.MustCompile(`(?i)\bpg_stat_file\s*\(`)},
	{"lo_import", regexp.MustCompile(`(?i)\blo_import\s*\(`)},
	{"lo_export", regexp.MustCompile(`(?i)\blo_export\s*\(`)},
	{"copy_program", regexp.MustCompile(`(?i)\bcopy\b[\s\S]{0,300}\bprogram\b`)},
	{"backslash_command", regexp.MustCompile(`(?m)^\s*\\`)},
}

// Check rejects SQL that tries to read the server filesystem or escape to a shell.
func Check(sql string) error {
	for _, p := range denied {
		if p.re.MatchString(sql) {
			return fmt.Errorf("sql uses disallowed construct %q", p.name)
		}
	}
	return nil
}

// Fingerprint is a stable hash of whitespace-normalized SQL.
func Fingerprint(sql string) string {
	norm := strings.Join(strings.Fields(sql), " ")
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// IsReadOnly reports whether the statement is a single read-only query that
// can be wrapped in EXPLAIN (ANALYZE, BUFFERS).
func IsReadOnly(sql string) bool {
	stripped := stripComments(sql)
	lower := strings.ToLower(stripped)
	for _, kw := range []string{
		"insert", "update", "delete", "drop", "alter", "truncate",
		"grant", "revoke", "copy", "create", "vacuum", "analyze",
		"call", "do", "merge",
	} {
		if containsWord(lower, kw) {
			return false
		}
	}
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "select", "with", "table", "values":
		return true
	default:
		return false
	}
}

func containsWord(sql, word string) bool {
	return regexp.MustCompile(`\b` + word + `\b`).MatchString(sql)
}

func stripComments(sql string) string {
	var b strings.Builder
	inSingle, inDouble, inLine, inBlock := false, false, false, false
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		if inLine {
			if c == '\n' {
				inLine = false
				b.WriteByte('\n')
			}
			continue
		}
		if inBlock {
			if c == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if inSingle {
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					b.WriteByte(sql[i+1])
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if inDouble {
			b.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
			continue
		}
		if c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			inLine = true
			i++
			continue
		}
		if c == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		if c == '\'' {
			inSingle = true
		} else if c == '"' {
			inDouble = true
		}
		b.WriteByte(c)
	}
	return b.String()
}

// Split divides a script into statements on semicolons that are outside
// quotes, comments, and dollar quotes.
func Split(sql string) ([]string, error) {
	var stmts []string
	var b strings.Builder
	inSingle, inDouble, inLine, inBlock := false, false, false, false
	var dollar string
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		if inLine {
			b.WriteByte(c)
			if c == '\n' {
				inLine = false
			}
			continue
		}
		if inBlock {
			b.WriteByte(c)
			if c == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				b.WriteByte(sql[i+1])
				i++
				inBlock = false
			}
			continue
		}
		if dollar != "" {
			if strings.HasPrefix(sql[i:], dollar) {
				b.WriteString(dollar)
				i += len(dollar) - 1
				dollar = ""
				continue
			}
			b.WriteByte(c)
			continue
		}
		if inSingle {
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					b.WriteByte(sql[i+1])
					i++
					continue
				}
				inSingle = false
			}
			continue
		}
		if inDouble {
			b.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
			continue
		}
		if c == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			b.WriteString("--")
			i++
			inLine = true
			continue
		}
		if c == '/' && i+1 < len(sql) && sql[i+1] == '*' {
			b.WriteString("/*")
			i++
			inBlock = true
			continue
		}
		if c == '\'' {
			inSingle = true
			b.WriteByte(c)
			continue
		}
		if c == '"' {
			inDouble = true
			b.WriteByte(c)
			continue
		}
		if c == '$' {
			if tag, ok := dollarTag(sql, i); ok {
				dollar = tag
				b.WriteString(tag)
				i += len(tag) - 1
				continue
			}
		}
		if c == ';' {
			stmt := strings.TrimSpace(b.String())
			if stmt != "" && !onlyComments(stmt) {
				stmts = append(stmts, stmt)
			}
			b.Reset()
			continue
		}
		b.WriteByte(c)
	}
	if inSingle || inDouble || dollar != "" || inBlock {
		return nil, fmt.Errorf("unterminated quote or comment in sql")
	}
	stmt := strings.TrimSpace(b.String())
	if stmt != "" && !onlyComments(stmt) {
		stmts = append(stmts, stmt)
	}
	return stmts, nil
}

func dollarTag(s string, i int) (string, bool) {
	if i >= len(s) || s[i] != '$' {
		return "", false
	}
	j := i + 1
	for j < len(s) && (unicode.IsLetter(rune(s[j])) || unicode.IsDigit(rune(s[j])) || s[j] == '_') {
		j++
	}
	if j < len(s) && s[j] == '$' {
		return s[i : j+1], true
	}
	return "", false
}

func onlyComments(stmt string) bool {
	return strings.TrimSpace(stripComments(stmt)) == ""
}
