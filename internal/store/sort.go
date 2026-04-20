package store

import (
	"strconv"
	"strings"
	"unicode"
)

func compareDisplayName(a, b string) int {
	aGroup, aSuffix := splitDisplayName(a)
	bGroup, bSuffix := splitDisplayName(b)

	if cmp := strings.Compare(aGroup, bGroup); cmp != 0 {
		return cmp
	}
	if cmp := naturalCompare(aSuffix, bSuffix); cmp != 0 {
		return cmp
	}
	return strings.Compare(a, b)
}

func splitDisplayName(name string) (string, string) {
	for i, r := range name {
		if isNameSeparator(r) {
			return name[:i], name[i+1:]
		}
	}
	return name, ""
}

func isNameSeparator(r rune) bool {
	switch r {
	case '_', '-', ' ', '\t':
		return true
	default:
		return false
	}
}

func naturalCompare(a, b string) int {
	aTokens := tokenizeNatural(a)
	bTokens := tokenizeNatural(b)
	limit := len(aTokens)
	if len(bTokens) < limit {
		limit = len(bTokens)
	}

	for i := 0; i < limit; i++ {
		cmp := compareNaturalToken(aTokens[i], bTokens[i])
		if cmp != 0 {
			return cmp
		}
	}

	switch {
	case len(aTokens) < len(bTokens):
		return -1
	case len(aTokens) > len(bTokens):
		return 1
	default:
		return 0
	}
}

type naturalToken struct {
	value    string
	isNumber bool
}

func tokenizeNatural(s string) []naturalToken {
	var tokens []naturalToken
	runes := []rune(s)
	for i := 0; i < len(runes); {
		r := runes[i]
		if isNameSeparator(r) {
			i++
			continue
		}

		isNumber := unicode.IsDigit(r)
		j := i + 1
		for j < len(runes) {
			next := runes[j]
			if isNameSeparator(next) || unicode.IsDigit(next) != isNumber {
				break
			}
			j++
		}

		token := string(runes[i:j])
		if !isNumber {
			token = strings.ToUpper(token)
		}
		tokens = append(tokens, naturalToken{
			value:    token,
			isNumber: isNumber,
		})
		i = j
	}
	return tokens
}

func compareNaturalToken(a, b naturalToken) int {
	if a.isNumber && b.isNumber {
		aValue := strings.TrimLeft(a.value, "0")
		bValue := strings.TrimLeft(b.value, "0")
		if aValue == "" {
			aValue = "0"
		}
		if bValue == "" {
			bValue = "0"
		}
		if len(aValue) != len(bValue) {
			if len(aValue) < len(bValue) {
				return -1
			}
			return 1
		}
		if aValue != bValue {
			if aValue < bValue {
				return -1
			}
			return 1
		}

		aInt, _ := strconv.Atoi(aValue)
		bInt, _ := strconv.Atoi(bValue)
		switch {
		case aInt < bInt:
			return -1
		case aInt > bInt:
			return 1
		default:
			return 0
		}
	}

	if a.isNumber != b.isNumber {
		if a.isNumber {
			return -1
		}
		return 1
	}

	return strings.Compare(a.value, b.value)
}
