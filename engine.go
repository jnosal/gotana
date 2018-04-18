package gotana

import (
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

const (
	STATE_INITIAL  = "INTITIAL"
	STATE_RUNNING  = "RUNNING"
	STATE_STOPPING = "STOPPING"
)

type Engine struct {
	state             string
	wg                sync.WaitGroup
	limitCrawl        int
	limitFail         int
	handler           ScrapingHandlerFunc
	finished          int
	scrapers          []*Scraper
	requestMiddleware []RequestMiddlewareFunc
	extensions        []Extension
	chDone            chan struct{}
	chScraped         chan ScrapedItem
	chItems           chan SaveableItem
	Meta              *EngineMeta
	Config            *ScraperConfig
}

func (engine *Engine) notifyExtensions(event string, prm extensionParameters) {
	go func() {
		switch event {
		case EVENT_SCRAPER_OPENED:
			for _, extension := range engine.extensions {
				extension.ScraperStarted(prm.scraper)
			}
		case EVENT_SCRAPER_CLOSED:
			for _, extension := range engine.extensions {
				extension.ScraperStopped(prm.scraper)
			}
		case EVENT_SAVEABLE_EXTRACTED:
			for _, extension := range engine.extensions {
				extension.ItemScraped(prm.scraper, prm.item)
			}
		default:
			panic("Inappropriate event: " + event)
		}
	}()
}

func (engine *Engine) SetHandler(handler ScrapingHandlerFunc) *Engine {
	engine.handler = handler
	return engine
}

func (engine *Engine) IncrFinishedCounter() {
	engine.finished += 1
}

func (engine Engine) Done() bool {
	return len(engine.scrapers) == engine.finished
}

func (engine *Engine) scrapingLoop() {
	Logger().Info("Starting scraping loop")

	for {
		select {
		case proxy, ok := <-engine.chScraped:
			if !ok {
				break
			}
			if engine.handler != nil {
				engine.handler(proxy, engine.chItems)
			}
			if proxy.scraper.handler != nil {
				proxy.scraper.handler(proxy, engine.chItems)
			}
		case item, ok := <-engine.chItems:
			if !ok {
				break
			}

			scraper := item.Scraper()
			engine.notifyExtensions(EVENT_SAVEABLE_EXTRACTED,
				extensionParameters{scraper: scraper, item: item})
		}
	}
}

func (engine *Engine) startTCPServer() {
	if engine.Config.TcpAddress != "" {
		server := NewTCPServer(engine.Config.TcpAddress, engine)
		server.Start()
	}
}

func (engine *Engine) startHTTPServer() {
	if engine.Config.HttpAddress != "" {
		server := NewHTTPServer(engine.Config.HttpAddress, engine)
		server.Start()
	}
}

func (engine *Engine) Start() {
	defer engine.Cleanup()
	Logger().Info("Starting engine")

	engine.state = STATE_RUNNING

	if len(engine.scrapers) == 0 {
		Logger().Warning("No scrapers have been registered. Exiting...")
		engine.Stop()
		return
	}

	for _, scraper := range engine.scrapers {
		go scraper.Start()
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go engine.startTCPServer()
	go engine.startHTTPServer()
	go engine.scrapingLoop()

	select {
	case <-engine.chDone:
		if engine.Done() {
			engine.wg.Wait()
			Logger().Warning("All scrapers have stopped. Exiting...")
			return
		}
	case sig := <-sigChan:
		Logger().Warningf("Got signal: %s. Gracefully stopping...", sig)
		engine.Stop()
		return
	}

}

func (engine *Engine) Stop() {
	engine.state = STATE_STOPPING
	Logger().Info("Stopping engine")

	for _, scraper := range engine.scrapers {
		scraper.Stop()
	}

	engine.wg.Wait()
}

func (engine *Engine) Cleanup() {
	close(engine.chDone)
	close(engine.chScraped)
	close(engine.chItems)
}

func (engine *Engine) HasScraper(name string) bool {
	for _, scraper := range engine.scrapers {
		if scraper.Name == name {
			return true
		}
	}
	return false
}

func (engine *Engine) GetScraper(name string) *Scraper {
	for _, scraper := range engine.scrapers {
		if scraper.Name == name {
			return scraper

		}
	}
	return nil
}

func (engine *Engine) AddScrapers(scrapers ...*Scraper) *Engine {
	for _, scraper := range scrapers {
		engine.Meta.ScraperStats[scraper.Name] = NewScraperMeta()
		scraper.engine = engine
		Logger().Debugf("Attached new scraper %s", scraper)
	}
	engine.scrapers = append(engine.scrapers, scrapers...)
	return engine
}

func (engine *Engine) UseMiddleware(middleware ...RequestMiddlewareFunc) *Engine {
	engine.requestMiddleware = append(engine.requestMiddleware, middleware...)
	return engine
}

func (engine *Engine) UseExtension(extensions ...Extension) *Engine {
	engine.extensions = append(engine.extensions, extensions...)
	return engine
}

func (engine *Engine) PrepareRequest(request *http.Request) *http.Request {
	for _, middleware := range engine.requestMiddleware {
		request = middleware(request)
	}
	return request
}

func (engine *Engine) FromConfig(config *ScraperConfig) *Engine {
	engine.Config = config

	for _, configData := range config.Scrapers {
		extractor := defaultExtractor()
		switch configData.Extractor {
		case "link":
			extractor = &LinkExtractor{}
		default:
			break
		}
		params := ScraperParams{
			Extractor:    extractor,
			Name:         configData.Name,
			Url:          configData.Url,
			RequestLimit: configData.RequestLimit,
		}
		scraper := NewScraper(params)
		for _, patternData := range configData.Patterns {
			pattern := NewURLPattern(patternData.Type, patternData.Pattern)
			scraper.AddPatterns(pattern)
		}
		Logger().Debugf("Defined following url patterns: %s", scraper.urlPatterns)
		engine.AddScrapers(scraper)
	}

	return engine
}

func GetDAO(engine *Engine) DAO {
	if engine.Config.RedisAddress != "" {
		return NewRedisDao(engine.Config.RedisAddress)
	}
	return nil
}

func NewEngine() (r *Engine) {
	r = &Engine{
		state:      STATE_INITIAL,
		Config:     NewSpiderConfig(""),
		Meta:       NewEngineMeta(),
		limitCrawl: 10000,
		limitFail:  500,
		finished:   0,
		chDone:     make(chan struct{}),
		chScraped:  make(chan ScrapedItem, 100),
		chItems:    make(chan SaveableItem, 250),
	}
	return
}
