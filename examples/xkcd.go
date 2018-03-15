package main

import (
	"encoding/json"
	"gotana"
	// "github.com/PuerkitoBio/goquery"
	"fmt"
	"github.com/PuerkitoBio/goquery"
)

type XkcdItem struct {
	gotana.ScraperMixin
	Img string
}

func (item XkcdItem) Validate() bool {
	return true
}

func (item XkcdItem) RecordData() ([]byte, error) {
	return json.Marshal(item)
}

func (item XkcdItem) String() (result string) {
	result = fmt.Sprintf("XKCD Image: %s", item.Img)
	return
}

func XkcdHandler(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	defer gotana.SilentRecover("HANDLER")
	document, err := proxy.HTMLDocument()
	if err != nil {
		gotana.Logger().Error(err.Error())
		return
	}
	document.Find("#comic img").Each(func(i int, s *goquery.Selection) {
		issue := XkcdItem{
			Img: s.AttrOr("src", ""),
		}
		issue.Proxy = proxy
		items <- issue
	})
}

func main() {
	config := gotana.NewSpiderConfig("xkcd.yml")
	engine := gotana.NewEngine()
	engine.FromConfig(config)
	engine.UseMiddleware(gotana.DelAcceptEncodingMiddleware).
		UseMiddleware(gotana.RandomUserAgentMiddleware).
		UseExtension(new(gotana.DisplayExtension))
	engine.GetScraper("xkcd").SetHandler(XkcdHandler)
	engine.Start()
}
