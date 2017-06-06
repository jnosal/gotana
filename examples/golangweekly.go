package main

import (
	"gotana"
	"strings"
)

type Item struct {
	gotana.ScraperMixin
}

func (item Item) Validate() bool {
	return true
}

func (item Item) RecordData() []string {
	return []string{"TEST", "DATA"}
}

func GlobalHandler(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	defer gotana.SilentRecover("HANDLER")

	if strings.Contains(proxy.Url, "/link") && strings.Contains(proxy.Url, "/web") {
		gotana.Logger().Debug(proxy)
		document, err := proxy.HTMLDocument()
		if err != nil {
			gotana.Logger().Error(err.Error())
			return
		}
		title := document.Find("title").First().Text()
		item := Item{}
		item.Proxy = proxy
		items <- item
		gotana.Logger().Noticef("%s --> %s", proxy.Url, title)
	}
}

func main() {
	config := gotana.NewSpiderConfig("sample1.yml")
	engine := gotana.NewEngine().SetHandler(GlobalHandler)
	engine.FromConfig(config)
	engine.UseMiddleware(gotana.DelAcceptEncodingMiddleware).
		UseMiddleware(gotana.RandomUserAgentMiddleware)
	engine.Run()
}
