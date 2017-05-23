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
	engine := gotana.NewEngine().SetHandler(GlobalHandler)
	engine.FromConfig(config)

	engine.Run()
}
