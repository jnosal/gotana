package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"gotana"
)

func GithubHandler(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	defer gotana.SilentRecover("HANDLER")
	document, err := proxy.HTMLDocument()
	if err != nil {
		gotana.Logger().Error(err.Error())
		return
	}
	document.Find(".repo-list li").Each(func(i int, s *goquery.Selection) {
		itemUrl := s.Find("h3 a").AttrOr("href", "")
		url := fmt.Sprintf("https://github.com/%s", itemUrl)
		gotana.Logger().Info(url)
	})
	proxy.ScheduleScraperStop()
}

func main() {
	engine := gotana.NewEngine()
	params := gotana.ScraperParams{
		Url:          "https://github.com/trending",
		RequestLimit: 1000,
	}
	scraper := gotana.NewScraper(params).SetHandler(GithubHandler)
	engine.AddScrapers(scraper)
	engine.Start()
}
