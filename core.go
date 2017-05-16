package gotana

import (
	"fmt"
	fury "github.com/jnosal/gofury"
	"golang.org/x/net/html"
	"io"
	"net"
	"net/http"
	URL "net/url"
	"strings"
	"sync"
	"time"
)


const (
	REQUEST_LIMIT_MILLISECOND = 100
	TIMEOUT_DIALER = time.Duration(time.Second * 10)
	TIMEOUT_REQUEST = time.Duration(time.Second * 10)
	TIMEOUT_TLS = time.Duration(time.Second * 5)
)

func GetHref(t html.Token) (ok bool, href string) {
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	return
}

type Extractable interface {
	Extract()
}

type LinkExtractor struct {
	Extractable
}

func (extractor *LinkExtractor) Extract(r io.ReadCloser, callback func(string)) {
	z := html.NewTokenizer(r)
	defer r.Close()

	for {
		tt := z.Next()

		switch {
		case tt == html.ErrorToken:
			return
		case tt == html.StartTagToken:
			t := z.Token()

			isAnchor := t.Data == "a"
			if !isAnchor {
				continue
			}

			ok, url := GetHref(t)
			if !ok {
				continue
			}
			callback(url)
		}
	}
}

type ScrapingResultProxy struct {
	resp    http.Response
	scraper Scraper
}

func (proxy ScrapingResultProxy) String() (result string) {
	result = "Twoja stara"
	return
}

type Runnable interface {
	Run() (err error)
}

type Engine struct {
	limitCrawl int
	limitFail  int
	handler    func(ScrapingResultProxy)
	finished   int
	scrapers   []*Scraper
	chDone     chan *Scraper
	chScraped  chan ScrapingResultProxy
}

func (engine *Engine) SetHandler(handler func(ScrapingResultProxy)) *Engine {
	engine.handler = handler
	return engine
}

func (engine *Engine) IncrFinishedCounter() {
	engine.finished += 1
}

func (engine Engine) Done() bool {
	return len(engine.scrapers) == engine.finished
}

func (engine *Engine) Run() {
	defer engine.Close()

	for _, scraper := range engine.scrapers {
		go scraper.Start()
	}

	// main scraping loop
	for {
		select {
		case proxy, ok := <-engine.chScraped:
			if !ok {
				break
			}
			if engine.handler != nil {
				engine.handler(proxy)
			}
			if proxy.scraper.handler != nil {
				proxy.scraper.handler(proxy)
			}

		case scraper, ok := <-engine.chDone:
			fury.Logger().Infof("Stopped %s", scraper)
			engine.IncrFinishedCounter()
			if !ok {
				break
			}
		}
		if engine.Done() {
			break
		}
	}
}

func (engine *Engine) Close() {
	close(engine.chDone)
	close(engine.chScraped)
}

func (engine *Engine) PushScraper(scrapers ...*Scraper) *Engine {
	for _, scraper := range scrapers {
		fury.Logger().Debugf("Attaching new scraper %s", scraper)
		scraper.engine = engine
	}
	engine.scrapers = append(engine.scrapers, scrapers...)
	return engine
}

type Scraper struct {
	crawled      int
	successful   int
	failed       int
	handler      func(ScrapingResultProxy)
	fetchMutex   *sync.Mutex
	crawledMutex *sync.Mutex
	domain       string
	baseUrl      string
	fetchedUrls  map[string]bool
	engine       *Engine
	extractor    *LinkExtractor
	chDone     chan struct{}
	chRequestUrl  chan string
}

func (scraper *Scraper) IncrCounters(isSuccessful bool) {
	scraper.crawledMutex.Lock()
	scraper.crawled += 1
	if isSuccessful {
		scraper.successful += 1
	} else {
		scraper.failed += 1
	}
	scraper.crawledMutex.Unlock()
}

func (scraper *Scraper) MarkAsFetched(url string) {
	scraper.fetchMutex.Lock()
	scraper.fetchedUrls[url] = true
	scraper.fetchMutex.Unlock()
}

func (scraper *Scraper) CheckIfShouldStop() (ok bool) {
	scraper.crawledMutex.Lock()
	if scraper.crawled == scraper.engine.limitCrawl {
		fury.Logger().Warningf("Crawl limit exceeded: %s", scraper)
		ok = true
	} else if scraper.failed == scraper.engine.limitFail {
		fury.Logger().Warningf("Fail limit exceeeded: %s", scraper)
		ok = true
	} else if scraper.failed == 1 && scraper.crawled == 1 {
		fury.Logger().Warningf("Base URL is corrupted: %s", scraper)
		ok = true
	}
	scraper.crawledMutex.Unlock()
	return
}

func (scraper *Scraper) CheckIfFetched(url string) (ok bool) {
	scraper.fetchMutex.Lock()
	_, ok = scraper.fetchedUrls[url]
	scraper.fetchMutex.Unlock()
	return
}

func (scraper *Scraper) CheckUrl(sourceUrl string) (ok bool, url string) {
	if strings.Contains(sourceUrl, scraper.domain) && strings.Index(sourceUrl, "http") == 0 {
		url = sourceUrl
		ok = true
	} else if strings.Index(sourceUrl, "/") == 0 {
		url = scraper.baseUrl + sourceUrl
		ok = true
	}
	return
}

func (scraper *Scraper) RunExtractor(resp *http.Response) {
	scraper.extractor.Extract(resp.Body, func(url string) {
		ok, url := scraper.CheckUrl(url)

		if ok {
			scraper.chRequestUrl <- url
		}
	})
}

func (scraper *Scraper) Stop() {
	fury.Logger().Infof("Stopping %s", scraper)
	scraper.chDone <- struct{}{}
	scraper.engine.chDone <- scraper
}

func (scraper *Scraper) Start() {
	fury.Logger().Infof("Starting: %s", scraper)
	scraper.chRequestUrl <- scraper.baseUrl

	limiter := time.Tick(time.Millisecond * REQUEST_LIMIT_MILLISECOND)

	for {
		select {
			case url := <-scraper.chRequestUrl:
				<-limiter
				go scraper.Fetch(url)
			case <-scraper.chDone:
				return
		}
	}
	return
}

func (scraper *Scraper) Notify(resp *http.Response) {
	scraper.engine.chScraped <- NewResultProxy(*scraper, *resp)
}

func (scraper *Scraper) Fetch(url string) (resp *http.Response, err error) {
	if ok := scraper.CheckIfFetched(url); ok {
		return
	}
	scraper.MarkAsFetched(url)

	fury.Logger().Infof("Fetching: %s", url)
	tic := time.Now()

	resp, err = NewHTTPClient().Get(url)

	fury.Logger().Debugf("Request to %s took: %s", url, time.Since(tic))

	scraper.IncrCounters(err == nil)

	if err == nil {
		scraper.Notify(resp)
		scraper.RunExtractor(resp)
	} else {
		fury.Logger().Warningf("Failed to crawl %s", url)
		fury.Logger().Debug(err)
	}

	if scraper.CheckIfShouldStop() {
		scraper.Stop()
	}
	return
}

func (scraper *Scraper) SetHandler(handler func(ScrapingResultProxy)) *Scraper {
	scraper.handler = handler
	return scraper
}

func (scraper *Scraper) String() (result string) {
	result = fmt.Sprintf("<Scraper: %s>. Crawled: %d, successful: %d failed: %d.",
		scraper.domain, scraper.crawled, scraper.successful, scraper.failed)
	return
}

func NewEngine() (r *Engine) {
	r = &Engine{
		limitCrawl: 1000,
		limitFail:  50,
		finished:   0,
		chDone:     make(chan *Scraper),
		chScraped:  make(chan ScrapingResultProxy),
	}
	return
}

func NewScraper(sourceUrl string) (s *Scraper) {
	parsed, err := URL.Parse(sourceUrl)
	if err != nil {
		fury.Logger().Infof("Inappropriate URL: %s", sourceUrl)
		return
	}
	s = &Scraper{
		crawled:      0,
		successful:   0,
		failed:       0,
		domain:       parsed.Host,
		baseUrl:      sourceUrl,
		fetchedUrls:  make(map[string]bool),
		crawledMutex: &sync.Mutex{},
		fetchMutex:   &sync.Mutex{},
		extractor:    &LinkExtractor{},
		chDone:     make(chan struct{}),
		chRequestUrl:  make(chan string, 1),
	}
	return
}

func NewResultProxy(scraper Scraper, resp http.Response) (result ScrapingResultProxy) {
	result = ScrapingResultProxy{scraper: scraper, resp: resp}
	return
}

func NewHTTPClient() (client *http.Client) {
	client = &http.Client{
		Timeout: TIMEOUT_REQUEST,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: TIMEOUT_DIALER,
			}).Dial,
			TLSHandshakeTimeout: TIMEOUT_TLS,
		},
	}
	return
}


type SpiderConfig struct {
	Project string `required:"true"`
	Spiders []struct {
		Name  string `required:"true"`
		Url string `required:"true"`
	}
}


func NewSpiderConfig(file string) (config *SpiderConfig) {
	config = &SpiderConfig{}
	fury.ProcessFile(config, file)
	return
}