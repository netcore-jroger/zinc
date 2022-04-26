package zutils

import (
	"strconv"
	"unicode"
)

func StringToInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func IsNumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
