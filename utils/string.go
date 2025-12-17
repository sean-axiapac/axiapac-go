package utils

import "fmt"

func Format[T any](ptr *T) string {
	if ptr == nil {
		return ""
	}
	return fmt.Sprintf("%v", *ptr)
}

func FormatBoolean(yesno bool, yes string, no string) string {
	if yesno {
		return yes
	}
	return no
}
