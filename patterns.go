package gotana

import (
	"fmt"
	"strings"
)

const TYPE_CONTAINS = "contains"

type URLPattern struct {
	Type    string `required:"true"`
	Pattern string `required:"true"`
}

func (item URLPattern) String() (result string) {
	result = fmt.Sprintf("URL Pattern [%s]: %s", item.Type, item.Pattern)
	return
}

func (item *URLPattern) Validate(url string) (result bool) {
	switch {
	case item.Type == TYPE_CONTAINS:
		result = strings.Contains(url, item.Pattern)
	default:
		result = false
	}
	return
}

func NewURLPattern(kind string, pattern string) (instance URLPattern) {
	instance = URLPattern{
		Type:    kind,
		Pattern: pattern,
	}
	return
}
