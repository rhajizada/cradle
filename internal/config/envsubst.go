package config

import (
	"os"
	"os/user"
	"strconv"
	"strings"
	"unicode"
)

// ExpandEnv expands:
//   - ${VAR}
//   - ${VAR:-default}   (default if unset OR empty)
//   - ${VAR-default}    (default if unset only)
//   - $VAR              (simple form)
//
// Escapes:
//   - \$VAR or \${VAR} keeps it literal
//   - $$ becomes a literal '$'
func ExpandEnv(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); {
		next, err := appendChunk(s, i, &b)
		if err != nil {
			return "", err
		}
		i = next
	}

	return b.String(), nil
}

const (
	braceOffset       = 2
	escapeTokenLength = 2
)

func appendChunk(s string, i int, b *strings.Builder) (int, error) {
	if isEscapedDollar(s, i) {
		b.WriteByte('$')
		return i + escapeTokenLength, nil
	}

	if isDoubleDollar(s, i) {
		b.WriteByte('$')
		return i + escapeTokenLength, nil
	}

	if s[i] != '$' {
		b.WriteByte(s[i])
		return i + 1, nil
	}

	if isBraceExpr(s, i) {
		return consumeBraceExpr(s, i, b)
	}

	return consumeSimpleVar(s, i, b), nil
}

func isEscapedDollar(s string, i int) bool {
	return s[i] == '\\' && i+1 < len(s) && s[i+1] == '$'
}

func isDoubleDollar(s string, i int) bool {
	return s[i] == '$' && i+1 < len(s) && s[i+1] == '$'
}

func isBraceExpr(s string, i int) bool {
	return i+1 < len(s) && s[i] == '$' && s[i+1] == '{'
}

func consumeBraceExpr(s string, i int, b *strings.Builder) (int, error) {
	end := strings.IndexByte(s[i+braceOffset:], '}')
	if end < 0 {
		return 0, ErrBadExpansion
	}
	end = i + braceOffset + end

	expr := s[i+braceOffset : end]
	val, ok, err := evalBraceExpr(expr)
	if err != nil {
		return 0, err
	}
	if ok {
		b.WriteString(val)
	}

	return end + 1, nil
}

func consumeSimpleVar(s string, i int, b *strings.Builder) int {
	j := i + 1
	if j >= len(s) || !isVarStart(rune(s[j])) {
		b.WriteByte('$')
		return i + 1
	}

	j++
	for j < len(s) && isVarContinue(rune(s[j])) {
		j++
	}

	key := s[i+1 : j]
	val, _ := lookupEnv(key)
	b.WriteString(val)
	return j
}

func evalBraceExpr(expr string) (string, bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", false, ErrBadExpansion
	}

	// Support:
	//   VAR
	//   VAR:-default
	//   VAR-default
	// Note: we do not implement nested parameter ops beyond this (keep simple).
	varName, op, rest := splitParamExpr(expr)

	if varName == "" {
		return "", false, ErrBadExpansion
	}

	cur, isSet := lookupEnv(varName)

	switch op {
	case "":
		return cur, true, nil
	case ":-":
		if !isSet || cur == "" {
			return rest, true, nil
		}
		return cur, true, nil
	case "-":
		if !isSet {
			return rest, true, nil
		}
		return cur, true, nil
	default:
		return "", false, ErrBadExpansion
	}
}

func splitParamExpr(expr string) (string, string, string) {
	// Find first '-' that participates in either ":-" or "-" after var name.
	// Var name ends at first non-var char.
	i := 0
	for i < len(expr) && isVarContinue(rune(expr[i])) {
		i++
	}
	varName := expr[:i]
	if i >= len(expr) {
		return varName, "", ""
	}

	// Expect "-" or ":-"
	var op string
	var rest string
	if expr[i] == '-' {
		op = "-"
		rest = expr[i+1:]
		return varName, op, rest
	}
	if expr[i] == ':' && i+1 < len(expr) && expr[i+1] == '-' {
		op = ":-"
		rest = expr[i+2:]
		return varName, op, rest
	}

	// Anything else is unsupported in this minimal expander.
	return "", "", ""
}

func isVarStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isVarContinue(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func lookupEnv(key string) (string, bool) {
	if v, ok := os.LookupEnv(key); ok {
		return v, true
	}
	switch key {
	case "UID":
		return strconv.Itoa(os.Getuid()), true
	case "GID":
		return strconv.Itoa(os.Getgid()), true
	case "USER":
		u, err := user.Current()
		if err == nil && u.Username != "" {
			return u.Username, true
		}
	}
	return "", false
}
