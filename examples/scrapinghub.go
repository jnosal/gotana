package main

import (
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"gotana"
)

type AItem struct {
	gotana.ScraperMixin
	Title string
}

func (item AItem) Validate() bool {
	return true
}

func (item AItem) RecordData() ([]byte, error) {
	return json.Marshal(item)
}

func ParseScrapingHub(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	defer gotana.SilentRecover("ParseScrapingHub")

	gotana.Logger().Debug(proxy)
	document, err := proxy.HTMLDocument()
	if err != nil {
		gotana.Logger().Error(err.Error())
		return
	}
	document.Find("h2.entry-title").Each(func(i int, s *goquery.Selection) {
		item := AItem{Title: gotana.StripString(s.Text())}
		item.SetProxy(proxy)
		items <- item
	})

}

func main() {
	config := gotana.NewSpiderConfig("scrapinghub.yml")
	engine := gotana.NewEngine()
	engine.FromConfig(config)
	engine.UseMiddleware(gotana.RandomUserAgentMiddleware)

	engine.GetScraper("scrapinghub").SetHandler(ParseScrapingHub)
	engine.Start()
}
