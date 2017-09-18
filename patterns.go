package gotana

import (
	"fmt"
	"regexp"
	"strings"
)

const TYPE_CONTAINS = "contains"
const TYPE_REGEXP = "regexp"

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
	case item.Type == TYPE_REGEXP:
		matched, _ := regexp.MatchString(item.Pattern, url)
		result = matched
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
