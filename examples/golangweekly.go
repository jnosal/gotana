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

type DummyExtension struct {
}

func (d *DummyExtension) ScraperStarted(scraper *gotana.Scraper) {
	gotana.Logger().Info("STARTED !")
}

func (d *DummyExtension) ScraperStopped(scraper *gotana.Scraper) {
	gotana.Logger().Info("STOPPED !")
}

func (d *DummyExtension) ItemScraped(scraper *gotana.Scraper, item gotana.SaveableItem) {
	gotana.Logger().Info("SCRAPED !")
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
	config := gotana.NewSpiderConfig("golangweekly.yml")
	engine := gotana.NewEngine().SetHandler(GlobalHandler)
	engine.FromConfig(config)
	engine.UseMiddleware(gotana.DelAcceptEncodingMiddleware).
		UseMiddleware(gotana.RandomUserAgentMiddleware).
		UseExtension(new(DummyExtension))
	engine.Start()
}
