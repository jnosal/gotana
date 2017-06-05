package main

import (
	"gotana"
	"strings"
)

type Item struct {
}

func (item Item) Validate() bool {
	return true
}

func (item Item) RecordData() []string {
	return []string{"TEST", "DATA"}
}

func GlobalHandler(proxy gotana.ScrapingResultProxy, items chan<- gotana.Saveable) {
	defer gotana.SilentRecover("HANDLER")

	if strings.Contains(proxy.Url, "/link") && strings.Contains(proxy.Url, "/web") {
		gotana.Logger().Debug(proxy)
		document, err := proxy.HTMLDocument()
		if err != nil {
			gotana.Logger().Error(err.Error())
			return
		}
		title := document.Find("title").First().Text()
		items <- Item{}
		gotana.Logger().Noticef("%s --> %s", proxy.Url, title)
	}
}

func main() {
	config := gotana.NewSpiderConfig("sample1.yml")
	engine := gotana.NewEngine().SetHandler(GlobalHandler)
	engine.FromConfig(config)
	engine.PushRequestMiddleware(gotana.DelAcceptEncodingMiddleware).
		PushRequestMiddleware(gotana.RandomUserAgentMiddleware)
	engine.Run()
}
