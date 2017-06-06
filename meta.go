package gotana

import (
	"net/http"
	"sync"
	"time"
)

type ScraperMeta struct {
	crawled    int
	successful int
	failed     int
	scraped    int
	saved      int
}

type EngineMeta struct {
	statsMutex    *sync.Mutex
	ScraperStats  map[string]*ScraperMeta
	Started       time.Time
	RequestsTotal int
	LastRequest   *http.Request
	LastResponse  *http.Response
}

func (meta *EngineMeta) IncrSaved(scraper *Scraper) {
	meta.statsMutex.Lock()
	defer meta.statsMutex.Unlock()
	stats := meta.ScraperStats[scraper.Name]
	stats.saved += 1
}

func (meta *EngineMeta) IncrScraped(scraper *Scraper) {
	meta.statsMutex.Lock()
	defer meta.statsMutex.Unlock()
	stats := meta.ScraperStats[scraper.Name]
	stats.scraped += 1
}

func (meta *EngineMeta) UpdateRequestStats(scraper *Scraper, isSuccessful bool, request *http.Request, response *http.Response) {
	meta.statsMutex.Lock()
	defer meta.statsMutex.Unlock()
	stats := meta.ScraperStats[scraper.Name]
	meta.RequestsTotal += 1
	meta.LastRequest = request
	meta.LastResponse = response

	stats.crawled += 1
	if isSuccessful {
		stats.successful += 1
	} else {
		stats.failed += 1
	}
}

func NewScraperMeta() (m *ScraperMeta) {
	m = &ScraperMeta{
		failed:     0,
		crawled:    0,
		successful: 0,
		scraped:    0,
		saved:      0,
	}
	return
}

func NewEngineMeta() (m *EngineMeta) {
	m = &EngineMeta{
		statsMutex:    &sync.Mutex{},
		RequestsTotal: 0,
		ScraperStats:  make(map[string]*ScraperMeta),
	}
	return
}
