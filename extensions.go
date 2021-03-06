package gotana

type Extension interface {
	ScraperStarted(scraper *Scraper)
	ScraperStopped(scraper *Scraper)
	ItemScraped(scraper *Scraper, item SaveableItem)
}

type SaveInRedisExtension struct {
}

func (d *SaveInRedisExtension) ScraperStarted(scraper *Scraper) {

}

func (d *SaveInRedisExtension) ScraperStopped(scraper *Scraper) {

}

func (d *SaveInRedisExtension) ItemScraped(scraper *Scraper, item SaveableItem) {
	if dao := GetDAO(scraper.engine); dao != nil {
		SaveItem(item, dao)
		scraper.engine.Meta.IncrSaved(scraper)
	}
}

type DisplayExtension struct {
}

func (d *DisplayExtension) ScraperStarted(scraper *Scraper) {

}

func (d *DisplayExtension) ScraperStopped(scraper *Scraper) {

}

func (d *DisplayExtension) ItemScraped(scraper *Scraper, item SaveableItem) {
	Logger().Noticef("%s", item)
}
