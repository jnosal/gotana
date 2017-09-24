package main

import (
	"gotana"
	"github.com/PuerkitoBio/goquery"
)

type Issue struct {
	gotana.ScraperMixin
	Title string
	Href  string
}

func (item Issue) Validate() bool {
	return true
}

func (item Issue) RecordData() []string {
	return []string{item.Title, item.Href}
}


func IssueHandler(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	defer gotana.SilentRecover("HANDLER")

	if proxy.CheckURLPatterns() {
		document, err := proxy.HTMLDocument()
		if err != nil {
			gotana.Logger().Error(err.Error())
			return
		}
		document.Find(".item-link-title a").Each(func(i int, s *goquery.Selection) {
			issue := Issue{
				Title: s.Text(),
				Href: s.AttrOr("href", ""),
			}
			issue.Proxy = proxy
			items <- issue
		})
	}
}

func main() {
	config := gotana.NewSpiderConfig("vue.yml")
	engine := gotana.NewEngine().SetHandler(IssueHandler)
	engine.FromConfig(config)
	engine.UseMiddleware(gotana.DelAcceptEncodingMiddleware).
		UseMiddleware(gotana.RandomUserAgentMiddleware)
	engine.Start()
}
