package common

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	}
}

func FormatBindingError(err error) string {
	if err == nil {
		return ""
	}

	if err == io.EOF {
		return "Request body is empty"
	}

	// Handle JSON syntax errors
	if syntaxErr, ok := err.(*json.SyntaxError); ok {
		return fmt.Sprintf("Invalid JSON at byte offset %d", syntaxErr.Offset)
	}

	// Handle JSON type errors (e.g. passing a string instead of a number)
	if typeErr, ok := err.(*json.UnmarshalTypeError); ok {
		return fmt.Sprintf("Field '%s' should be of type %s", typeErr.Field, typeErr.Type.String())
	}

	// Handle validation errors
	if ve, ok := err.(validator.ValidationErrors); ok {
		var out []string
		for _, fe := range ve {
			out = append(out, formatFieldError(fe))
		}
		return strings.Join(out, ", ")
	}

	// Generic fallback
	return err.Error()
}

func formatFieldError(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("Field '%s' is required", fe.Field())
	case "email":
		return fmt.Sprintf("Field '%s' must be a valid email", fe.Field())
	case "url":
		return fmt.Sprintf("Field '%s' must be a valid URL", fe.Field())
	case "min":
		return fmt.Sprintf("Field '%s' must be at least %s", fe.Field(), fe.Param())
	case "max":
		return fmt.Sprintf("Field '%s' must be at most %s", fe.Field(), fe.Param())
	case "numeric":
		return fmt.Sprintf("Field '%s' must be numeric", fe.Field())
	case "alphanum":
		return fmt.Sprintf("Field '%s' must be alphanumeric", fe.Field())
	case "len":
		return fmt.Sprintf("Field '%s' must have length %s", fe.Field(), fe.Param())
	}
	return fmt.Sprintf("Field '%s' failed validation for '%s'", fe.Field(), fe.Tag())
}
