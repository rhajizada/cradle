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
		ch := s[i]

		// Escape: \$
		if ch == '\\' && i+1 < len(s) && s[i+1] == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}

		// $$ -> $
		if ch == '$' && i+1 < len(s) && s[i+1] == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}

		if ch != '$' {
			b.WriteByte(ch)
			i++
			continue
		}

		// ${...}
		if i+1 < len(s) && s[i+1] == '{' {
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				return "", ErrBadExpansion
			}
			end = i + 2 + end // absolute index of '}'

			expr := s[i+2 : end] // inside braces
			val, ok, err := evalBraceExpr(expr)
			if err != nil {
				return "", err
			}
			if ok {
				b.WriteString(val)
			}
			i = end + 1
			continue
		}

		// $VAR
		j := i + 1
		if j >= len(s) || !(isVarStart(rune(s[j]))) {
			// Just a lone '$'
			b.WriteByte('$')
			i++
			continue
		}
		j++
		for j < len(s) && isVarContinue(rune(s[j])) {
			j++
		}
		key := s[i+1 : j]
		val, _ := lookupEnv(key)
		b.WriteString(val)
		i = j
	}

	return b.String(), nil
}

func evalBraceExpr(expr string) (val string, ok bool, err error) {
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
		// default if unset OR empty
		if !isSet || cur == "" {
			return rest, true, nil
		}
		return cur, true, nil
	case "-":
		// default if unset only
		if !isSet {
			return rest, true, nil
		}
		return cur, true, nil
	default:
		return "", false, ErrBadExpansion
	}
}

func splitParamExpr(expr string) (varName string, op string, rest string) {
	// Find first '-' that participates in either ":-" or "-" after var name.
	// Var name ends at first non-var char.
	i := 0
	for i < len(expr) && isVarContinue(rune(expr[i])) {
		i++
	}
	varName = expr[:i]
	if i >= len(expr) {
		return varName, "", ""
	}

	// Expect "-" or ":-"
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
