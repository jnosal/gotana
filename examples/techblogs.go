package main

import (
	//"github.com/PuerkitoBio/goquery"
	"gotana"
	"strings"
)

func ParsePornHub(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	defer gotana.SilentRecover("ParsePornHub")

	if strings.Contains(proxy.Url, "/insights/") {

		document, err := proxy.HTMLDocument()
		if err != nil {
			gotana.Logger().Error(err.Error())
			return
		}
		title := document.Find("title").First().Text()

		if title != "" {
			gotana.Logger().Noticef("%s --> %s", proxy.Url, title)
		}

	}
}

func ParseSpotify(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	defer gotana.SilentRecover("ParseSpotify")

	if gotana.ContainsOneOf(proxy.Url, []string{"2017", "2016", "2015", "2014"}) {
		gotana.Logger().Debug(proxy)
		document, err := proxy.HTMLDocument()
		if err != nil {
			gotana.Logger().Error(err.Error())
			return
		}
		title := document.Find("h1.blog-post-title").First().Text()
		if title != "" {
			gotana.Logger().Noticef("%s --> %s", proxy.Url, title)
		}
	}
}

func main() {
	config := gotana.NewSpiderConfig("techblogs.yml")
	engine := gotana.NewEngine()
	engine.FromConfig(config)
	engine.UseMiddleware(gotana.RandomUserAgentMiddleware)

	//engine.GetScraper("pornhub").SetHandler(ParsePornHub)
	engine.GetScraper("spotify").SetHandler(ParseSpotify)

	engine.Start()
}
