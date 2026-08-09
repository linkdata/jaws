package ui

import (
	"regexp"
	"strconv"
	"strings"
)

var htmlNumberPattern = regexp.MustCompile(`^-?([0-9]+(\.[0-9]+)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)

func validHTMLNumber(text string) bool {
	return htmlNumberPattern.MatchString(text)
}

// htmlIntegerText converts text accepted by validHTMLNumber to decimal integer
// syntax and rejects values with a nonzero fractional part.
func htmlIntegerText(text string, signed bool) (result string, ok bool) {
	negative := text[0] == '-'
	if negative {
		text = text[1:]
	}

	mantissa := text
	exponentText := ""
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		mantissa, exponentText = text[:i], text[i+1:]
	}

	fractionDigits := 0
	digits := mantissa
	if i := strings.IndexByte(mantissa, '.'); i >= 0 {
		fractionDigits = len(mantissa) - i - 1
		digits = mantissa[:i] + mantissa[i+1:]
	}
	digitCount := len(digits)
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return "0", true
	}
	if negative && !signed {
		return
	}

	significant := strings.TrimRight(digits, "0")
	trailingZeros := len(digits) - len(significant)
	// No Go integer has more than 20 decimal digits. Use the full, pre-trim
	// digit count so the exponent can still cancel leading fractional zeroes.
	exponent := boundedHTMLExponent(exponentText, digitCount+21)
	zeros := exponent - fractionDigits + trailingZeros
	if zeros < 0 || len(significant)+zeros > 20 {
		return "", false
	}
	result = significant + strings.Repeat("0", zeros)
	if negative {
		result = "-" + result
	}
	return result, true
}

func boundedHTMLExponent(text string, limit int) int {
	if text == "" {
		return 0
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		if text[0] == '-' {
			return -limit
		}
		return limit
	}
	if value < -int64(limit) {
		return -limit
	}
	if value > int64(limit) {
		return limit
	}
	return int(value)
}
