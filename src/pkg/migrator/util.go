package migrator

import (
	"strings"
)

func compareVersions(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := max(len(aParts), len(bParts))
	for i := 0; i < maxLen; i++ {
		aVal := 0
		if i < len(aParts) {
			aVal = atoi(aParts[i])
		}

		bVal := 0
		if i < len(bParts) {
			bVal = atoi(bParts[i])
		}

		if aVal < bVal {
			return -1
		} else if aVal > bVal {
			return 1
		}
	}
	return 0
}

func atoi(s string) int {
	res := 0
	for _, c := range s {
		res = res*10 + int(c-'0')
	}
	return res
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// normalizeLineEndings converts CRLF / CR to LF,
// so scripts saved on Windows are not rejected outright.
func normalizeLineEndings(sql string) string {
	sql = strings.ReplaceAll(sql, "\r\n", "\n")
	sql = strings.ReplaceAll(sql, "\r", "\n")
	return sql
}

// isExecutableCommentStart reports whether sql[i:] starts with a MySQL
// executable comment "/*!" or "/*!NNNNN". Such comments are executed by
// MySQL as if the surrounding markers were removed, so the migration engine
// must forward them verbatim rather than discarding them.
//
// Optimizer hints ("/*+ ... */") are NOT considered executable comments
// here: MySQL only honors them in specific statement positions, and dropping
// them during migration is harmless. They are treated as ordinary block
// comments.
func isExecutableCommentStart(sql string, i int) bool {
	// Precondition: sql[i:i+2] == "/*"
	return i+2 < len(sql) && sql[i+2] == '!'
}

// discardBlockComment consumes an ordinary /* ... */ block comment starting
// at i (where sql[i:i+2] == "/*"), returning the index just after the
// closing "*/". Unterminated comments consume the rest of the input.
//
// MySQL does not support nested block comments: the first "*/" ends the
// comment. This scanner follows the same rule.
func discardBlockComment(sql string, i int) int {
	n := len(sql)
	i += 2 // skip "/*"
	for i+1 < n && !(sql[i] == '*' && sql[i+1] == '/') {
		i++
	}
	if i+1 < n {
		return i + 2 // skip "*/"
	}
	return n
}

// copyExecutableComment appends the executable comment starting at i
// (sql[i:i+2] == "/*", sql[i+2] == '!') to buf and returns the index just
// after the closing "*/". Unterminated comments consume the rest of input.
func copyExecutableComment(buf *strings.Builder, sql string, i int) int {
	n := len(sql)
	buf.WriteString("/*")
	i += 2 // skip "/*"
	for i < n {
		if sql[i] == '*' && i+1 < n && sql[i+1] == '/' {
			buf.WriteString("*/")
			return i + 2
		}
		buf.WriteByte(sql[i])
		i++
	}
	return n
}

// splitSQL uses a state machine to split a script into individual SQL statements.
//
// Compared to the old strings.Split(sql, ";"), this implementation correctly handles:
//   - Semicolons and comment markers inside single-quoted / double-quoted / backtick string literals
//   - --  line comments (MySQL requires -- to be followed by whitespace or end-of-line; this is checked)
//   - #   line comments (MySQL)
//   - Ordinary /* ... */ block comments: discarded verbatim
//   - MySQL executable comments /*! ... */: preserved verbatim so the
//     database executes the embedded SQL (e.g. SET NAMES, CREATE TRIGGER)
//   - A final statement without a trailing semicolon
//
// Returned statements do not include the trailing semicolon (it is consumed as a separator).
// Line comment handling: skip to just before '\n' without writing '\n' ourselves;
// the main loop writes the single '\n' on the next iteration, avoiding double newlines (blank lines).
//
// Note: statements containing BEGIN...END blocks (stored procedures, triggers, functions)
// will still be split on internal semicolons. Supporting those requires DELIMITER
// or a dedicated parser, which this implementation does not provide.
func splitSQL(sql string) []string {
	sql = normalizeLineEndings(sql)

	var stmts []string
	var buf strings.Builder
	i, n := 0, len(sql)

	flush := func() {
		if s := strings.TrimSpace(buf.String()); s != "" {
			stmts = append(stmts, s)
		}
		buf.Reset()
	}

	for i < n {
		c := sql[i]
		switch {
		// String literals: '...' / "..." / `...`
		case c == '\'' || c == '"' || c == '`':
			quote := c
			buf.WriteByte(c)
			i++
			for i < n {
				buf.WriteByte(sql[i])
				if sql[i] == '\\' && i+1 < n { // handle escape characters, e.g. \'
					i++
					buf.WriteByte(sql[i])
					i++
					continue
				}
				if sql[i] == quote {
					// In MySQL, '' represents an escaped single quote; check again.
					if quote != '`' && i+1 < n && sql[i+1] == quote {
						buf.WriteByte(sql[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}

		// -- line comment (MySQL requires whitespace or EOL after --; otherwise treat as minus signs)
		case c == '-' && i+1 < n && sql[i+1] == '-':
			if i+2 >= n || sql[i+2] == ' ' || sql[i+2] == '\t' || sql[i+2] == '\n' {
				// Skip to EOL without writing '\n'; the main loop will write the single newline.
				for i < n && sql[i] != '\n' {
					i++
				}
			} else {
				buf.WriteByte(c)
				i++
			}

		// # line comment (MySQL)
		case c == '#':
			// Also skip to EOL without writing '\n', avoiding blank lines.
			for i < n && sql[i] != '\n' {
				i++
			}

		// /* ... */ block comment.
		//
		// MySQL executable comments (/*! ... */) MUST be preserved so the
		// database sees and executes the embedded SQL. Ordinary block
		// comments (including optimizer hints /*+ ... */) are discarded.
		case c == '/' && i+1 < n && sql[i+1] == '*':
			if isExecutableCommentStart(sql, i) {
				i = copyExecutableComment(&buf, sql, i)
			} else {
				i = discardBlockComment(sql, i)
				// Write a space so adjacent tokens do not get concatenated
				// when a comment sits between them ("SELECT/*x*/1" -> "SELECT 1").
				buf.WriteByte(' ')
			}

		// Statement separator
		case c == ';':
			flush()
			i++

		default:
			buf.WriteByte(c)
			i++
		}
	}

	// Final statement without a trailing semicolon
	flush()

	return stmts
}
