package main

import (
	"gotana"
	"strings"
	"github.com/PuerkitoBio/goquery"
)

func GlobalHandler(proxy gotana.ScrapingResultProxy) {
	defer gotana.SilentRecover("HANDLER")

	if strings.Contains(proxy.Url, "/link") && strings.Contains(proxy.Url, "/web") {
		defer proxy.Response.Body.Close()
		gotana.Logger().Debug(proxy)

		document, err := goquery.NewDocumentFromReader(proxy.Response.Body)

		if err != nil {
			gotana.Logger().Error(err.Error())
			return
		}
		title := document.Find("title").First().Text()
		gotana.Logger().Noticef("%s --> %s", proxy.Url, title)
	}
}


func main() {
	config := gotana.NewSpiderConfig("data.yml")
	engine := gotana.NewEngine().SetHandler(GlobalHandler)
	engine.FromConfig(config)

	engine.Run()
	//r, _ := http.Get("https://golangweekly.com/link/14366/web")
	//fmt.Println(r.StatusCode)
}
