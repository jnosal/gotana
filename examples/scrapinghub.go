package main

import (
	"github.com/PuerkitoBio/goquery"
	"gotana"
)

type AItem struct {
	gotana.ScraperMixin
}

func (item AItem) Validate() bool {
	return true
}

func (item AItem) RecordData() []string {
	return []string{"TEST", "DATA"}
}

func ParseScrapingHub(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	defer gotana.SilentRecover("ParseScrapingHub")

	gotana.Logger().Debug(proxy)
	document, err := proxy.HTMLDocument()
	if err != nil {
		gotana.Logger().Error(err.Error())
		return
	} else {
		gotana.Logger().Info("DUPA")
	}
	document.Find("h2.entry-title").Each(func(i int, s *goquery.Selection) {
		gotana.Logger().Notice(gotana.StripString(s.Text()))
	})

}

func main() {
	config := gotana.NewSpiderConfig("scrapinghub.yml")
	engine := gotana.NewEngine()
	engine.FromConfig(config)
	engine.UseMiddleware(gotana.RandomUserAgentMiddleware)

	engine.GetScraper("scrapinghub").SetHandler(ParseScrapingHub)
	engine.Run()
}
