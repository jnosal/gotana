package main

import (
	"fmt"
	"gotana"
)


func GlobalHandler(proxy gotana.ScrapingResultProxy) {
	fmt.Println("D")
}

func ScraperHandler(proxy gotana.ScrapingResultProxy) {
	fmt.Println("DWWW")
}

func main() {
	config := gotana.NewSpiderConfig("data.yml")
	fmt.Println(config)
	engine := gotana.NewEngine().SetHandler(GlobalHandler)

	engine.PushScraper(gotana.NewScraper("http://golangweekly!!!.com").SetHandler(ScraperHandler))
	engine.PushScraper(gotana.NewScraper("http://golangweekly.com").SetHandler(ScraperHandler))
	engine.Run()
}
