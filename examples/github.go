package main

import (
	//"encoding/json"
	//"github.com/PuerkitoBio/goquery"
	"gotana"
)

func main() {
	engine := gotana.NewEngine()
	params := gotana.ScraperParams{
		Url:          "https://github.com/trending",
		RequestLimit: 1000,
	}
	scraper := gotana.NewScraper(params)
	engine.AddScrapers(scraper)
	engine.Start()
}
