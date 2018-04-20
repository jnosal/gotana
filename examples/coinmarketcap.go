package main

import (
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"gotana"
)

type CoinMarketCapItem struct {
	gotana.ScraperMixin
	Name       string
	Symbol     string
	Price      string
	Volume     string
	Cap        string
	Percent1h  string
	Percent24h string
	Percent7d  string
}

func (item CoinMarketCapItem) Validate() bool {
	return true
}

func (item CoinMarketCapItem) RecordData() ([]byte, error) {
	return json.Marshal(item)
}

func CoinMarketCapHandler(proxy gotana.ScrapedItem, items chan<- gotana.SaveableItem) {
	if document, err := proxy.HTMLDocument(); err == nil {
		document.Find("#currencies-all tbody tr").Each(func(i int, s *goquery.Selection) {
			item := CoinMarketCapItem{
				Name:       s.Find(".currency-name-container").Text(),
				Symbol:     s.Find(".col-symbol").Text(),
				Price:      s.Find("a.price").AttrOr("data-usd", ""),
				Volume:     s.Find("a.volume").AttrOr("data-usd", ""),
				Cap:        s.Find(".market-cap").AttrOr("data-usd", ""),
				Percent1h:  s.Find(".percent-1h").Text(),
				Percent24h: s.Find(".percent-24h").Text(),
				Percent7d:  s.Find(".prcent-7d").Text(),
			}
			item.Proxy = proxy
			go func() {
				items <- item
			}()
		})
	}
}

func main() {
	config := gotana.NewSpiderConfig("coinmarketcap.yml")
	engine := gotana.NewEngine().SetHandler(CoinMarketCapHandler)
	engine.FromConfig(config).
		UseExtension(new(gotana.SaveInRedisExtension))
	engine.Start()
}
