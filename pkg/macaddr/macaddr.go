// Package macaddr provides utilities for working with MAC addresses including
// normalization, formatting, pattern matching, and validation.
package macaddr

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// NormalizeExactMac normalizes a MAC address to a 12-character lowercase hex string
// without separators. Accepts colon, dot, or no separators.
// Returns an error if the input is not a valid MAC address.
func NormalizeExactMac(input string) (string, error) {
	clean := strings.Map(func(r rune) rune {
		if r == ':' || r == '.' || r == '-' {
			return -1
		}
		return r
	}, strings.ToLower(input))

	if len(clean) != 12 {
		return "", fmt.Errorf("invalid MAC address length: %s", input)
	}
	for _, r := range clean {
		if !isHexDigit(byte(r)) {
			return "", fmt.Errorf("invalid MAC address characters: %s", input)
		}
	}
	return clean, nil
}

// FormatMacColon formats a normalized 12-character MAC address with colon separators.
// Example: "001122334455" -> "00:11:22:33:44:55"
func FormatMacColon(clean string) string {
	clean = strings.ToLower(clean)
	if len(clean) != 12 {
		return clean
	}
	var b strings.Builder
	for i := 0; i < 12; i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(clean[i : i+2])
	}
	return b.String()
}

// NormalizePatternInput normalizes a MAC pattern by removing separators
// but preserving wildcards (*) and bracket patterns ([...]).
// Note: * remains as *, representing one byte (2 hex chars)
func NormalizePatternInput(input string) string {
	var b strings.Builder
	for _, r := range input {
		switch r {
		case ':', '.', '-':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToUpper(b.String())
}

// BuildMacMatcher creates a MAC matching function from an input pattern.
// Returns:
//   - matcher function that tests if a MAC matches the pattern
//   - normalized pattern string
//   - isPattern flag (true for wildcards, false for exact match)
//   - error if the pattern is invalid
//
// Supports:
//   - Exact MAC: "00:11:22:33:44:55"
//   - Wildcards: "00:11:22:33:44:*" where * matches one byte
//   - Bracket patterns: "00:11:22:33:44:[1-4][0-f]" for hex ranges
func BuildMacMatcher(input string) (func(string) bool, string, bool, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, "", false, errors.New("MAC pattern cannot be empty")
	}

	hasWildcard := strings.Contains(input, "*") || strings.Contains(input, "[")
	if !hasWildcard {
		normalized, err := NormalizeExactMac(input)
		if err != nil {
			return nil, "", false, err
		}
		displayMac := FormatMacColon(normalized)
		return func(mac string) bool {
			return strings.EqualFold(mac, normalized)
		}, displayMac, false, nil
	}

	clean := NormalizePatternInput(input)
	re, err := BuildMacRegex(clean)
	if err != nil {
		return nil, "", false, err
	}
	return func(mac string) bool {
		normalized, err := NormalizeExactMac(mac)
		if err != nil {
			return false
		}
		return re.MatchString(strings.ToUpper(normalized))
	}, input, true, nil
}

// BuildMacRegex builds a regex pattern from a normalized MAC pattern string.
// The pattern should be uppercase and have separators removed.
// Example: "0011223344**" or "0011223344[1-4][0-F]"
func BuildMacRegex(clean string) (*regexp.Regexp, error) {
	var b strings.Builder
	nibbleCount := 0
	i := 0
	for i < len(clean) {
		switch clean[i] {
		case '[':
			end := strings.Index(clean[i:], "]")
			if end == -1 {
				return nil, errors.New("unmatched bracket in MAC pattern")
			}
			token := clean[i : i+end+1]
			sanitized, err := sanitizeBracket(token)
			if err != nil {
				return nil, err
			}
			b.WriteString(sanitized)
			nibbleCount++
			i += end + 1
		case '*':
			b.WriteString("[0-9A-F]{2}")
			nibbleCount += 2
			i++
		default:
			if !isHexDigit(clean[i]) {
				return nil, fmt.Errorf("invalid MAC pattern: %s", clean)
			}
			b.WriteByte(clean[i])
			nibbleCount++
			i++
		}
	}
	if nibbleCount != 12 {
		return nil, fmt.Errorf("invalid MAC pattern length (need 12 nibbles): %s", clean)
	}
	return regexp.Compile("^" + b.String() + "$")
}

// sanitizeBracket validates and normalizes a bracket pattern token like "[1-4]" or "[0-F]".
// Returns the sanitized pattern in uppercase or an error if the pattern is invalid.
func sanitizeBracket(token string) (string, error) {
	if !strings.HasPrefix(token, "[") || !strings.HasSuffix(token, "]") {
		return "", errors.New("invalid bracket pattern")
	}
	inner := strings.ToUpper(token[1 : len(token)-1])
	if inner == "" {
		return "", errors.New("empty bracket pattern")
	}
	for _, r := range inner {
		if r == '-' {
			continue
		}
		if !isHexDigit(byte(r)) {
			return "", fmt.Errorf("invalid bracket pattern: %s", token)
		}
	}
	return "[" + inner + "]", nil
}

// isHexDigit checks if a byte is a valid hexadecimal digit (0-9, A-F, a-f).
func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')
}
