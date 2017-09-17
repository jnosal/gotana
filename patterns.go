package gotana

import "fmt"

type URLPattern struct {
	Type    string `required:"true"`
	Pattern string `required:"true"`
}

func (item URLPattern) String() (result string) {
	result = fmt.Sprintf("URL Pattern [%s]: %s", item.Type, item.Pattern)
	return
}

func NewURLPattern(kind string, pattern string) (instance URLPattern) {
	instance = URLPattern{
		Type:    kind,
		Pattern: pattern,
	}
	return
}
