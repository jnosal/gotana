package gotana

import (
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net"
	"net/http"
	URL "net/url"
	"strings"
	"sync"
	"time"
	"github.com/PuerkitoBio/goquery"
)

const (
	REQUEST_LIMIT_MILLISECOND = 100
	TIMEOUT_DIALER            = time.Duration(time.Second * 30)
	TIMEOUT_REQUEST           = time.Duration(time.Second * 30)
	TIMEOUT_TLS               = time.Duration(time.Second * 10)
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
	Url 	string
	Response    http.Response
	scraper Scraper
}

func (proxy ScrapingResultProxy) String() (result string) {
	result = fmt.Sprintf("Result of scraping: %s", proxy.Url)
	return
}

func (proxy ScrapingResultProxy) CheckIfRedirected() bool {
	return proxy.Url != proxy.Response.Request.URL.String()
}

func (proxy ScrapingResultProxy) finalResponseBody() (io.ReadCloser, error) {
	if proxy.CheckIfRedirected() {
		client := NewHTTPClient()
		response, err := client.Get(proxy.Response.Request.URL.String())
		if err != nil {
			return nil, err
		}
		return response.Body, nil
	}
	return proxy.Response.Body, nil
}

func (proxy ScrapingResultProxy) HTMLDocument() (document *goquery.Document, err error) {
	responseBody, err := proxy.finalResponseBody()

	if err == nil {
		document, err = goquery.NewDocumentFromReader(responseBody)
	}

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
			Logger().Infof("Stopped %s", scraper)
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
		Logger().Debugf("Attaching new scraper %s", scraper)
		scraper.engine = engine
	}
	engine.scrapers = append(engine.scrapers, scrapers...)
	return engine
}

func (engine *Engine) FromConfig(config *SpiderConfig) *Engine {
	for _, data := range config.Spiders {
		scraper := NewScraper(data.Name, data.Url)
		engine.PushScraper(scraper)
	}

	return engine
}

type Scraper struct {
	crawled      int
	successful   int
	failed       int
	handler      func(ScrapingResultProxy)
	fetchMutex   *sync.Mutex
	crawledMutex *sync.Mutex
	name         string
	domain       string
	baseUrl      string
	fetchedUrls  map[string]bool
	engine       *Engine
	extractor    *LinkExtractor
	chDone       chan struct{}
	chRequestUrl chan string
}

func (scraper *Scraper) IncrCounters(isSuccessful bool) {
	scraper.crawledMutex.Lock()
	defer scraper.crawledMutex.Unlock()

	scraper.crawled += 1
	if isSuccessful {
		scraper.successful += 1
	} else {
		scraper.failed += 1
	}
}

func (scraper *Scraper) MarkAsFetched(url string) {
	scraper.fetchMutex.Lock()
	defer scraper.fetchMutex.Unlock()

	scraper.fetchedUrls[url] = true
}

func (scraper *Scraper) CheckIfShouldStop() (ok bool) {
	scraper.crawledMutex.Lock()
	defer scraper.crawledMutex.Unlock()

	if scraper.crawled == scraper.engine.limitCrawl {
		Logger().Warningf("Crawl limit exceeded: %s", scraper)
		ok = true
	} else if scraper.failed == scraper.engine.limitFail {
		Logger().Warningf("Fail limit exceeeded: %s", scraper)
		ok = true
	} else if scraper.failed == 1 && scraper.crawled == 1 {
		Logger().Warningf("Base URL is corrupted: %s", scraper)
		ok = true
	}
	return
}

func (scraper *Scraper) CheckIfFetched(url string) (ok bool) {
	scraper.fetchMutex.Lock()
	defer scraper.fetchMutex.Unlock()

	_, ok = scraper.fetchedUrls[url]
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

func (scraper *Scraper) RunExtractor(resp http.Response) {
	defer SilentRecover("EXTRACTOR")

	scraper.extractor.Extract(resp.Body, func(url string) {
		ok, url := scraper.CheckUrl(url)

		if ok {
			scraper.chRequestUrl <- url
		}
	})
}

func (scraper *Scraper) Stop() {
	Logger().Infof("Stopping %s", scraper)
	scraper.chDone <- struct{}{}
	scraper.engine.chDone <- scraper
}

func (scraper *Scraper) Start() {
	Logger().Infof("Starting: %s", scraper)
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

func (scraper *Scraper) Notify(url string, resp *http.Response) {
	scraper.engine.chScraped <- NewResultProxy(url, *scraper, *resp)
}

func (scraper *Scraper) Fetch(url string) (resp *http.Response, err error) {
	if ok := scraper.CheckIfFetched(url); ok {
		return
	}
	scraper.MarkAsFetched(url)

	Logger().Infof("Fetching: %s", url)
	tic := time.Now()
	request, _ := http.NewRequest("GET", url, nil)
	request.Header.Del("Accept-Encoding")

	resp, err = NewHTTPClient().Do(request)

	statusCode := 0
	if err == nil {
		statusCode = resp.StatusCode
	}

	Logger().Debugf("[%d]Request to %s took: %s", statusCode, url, time.Since(tic))

	scraper.IncrCounters(err == nil)

	if err == nil {
		scraper.Notify(url, resp)
		scraper.RunExtractor(*resp)
	} else {
		Logger().Warningf("Failed to crawl %s", url)
		Logger().Warning(err)
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
		limitCrawl: 10000,
		limitFail:  500,
		finished:   0,
		chDone:     make(chan *Scraper),
		chScraped:  make(chan ScrapingResultProxy),
	}
	return
}

func NewScraper(name string, sourceUrl string) (s *Scraper) {
	parsed, err := URL.Parse(sourceUrl)
	if err != nil {
		Logger().Infof("Inappropriate URL: %s", sourceUrl)
		return
	}
	s = &Scraper{
		crawled:      0,
		successful:   0,
		failed:       0,
		name:         name,
		domain:       parsed.Host,
		baseUrl:      sourceUrl,
		fetchedUrls:  make(map[string]bool),
		crawledMutex: &sync.Mutex{},
		fetchMutex:   &sync.Mutex{},
		extractor:    &LinkExtractor{},
		chDone:       make(chan struct{}),
		chRequestUrl: make(chan string, 5),
	}
	return
}

func NewResultProxy(url string, scraper Scraper, resp http.Response)  ScrapingResultProxy {
	return ScrapingResultProxy{
		Response: resp,
		Url: url,
		scraper: scraper,
	}
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
		Name string `required:"true"`
		Url  string `required:"true"`
	}
}

func NewSpiderConfig(file string) (config *SpiderConfig) {
	config = &SpiderConfig{}
	ProcessFile(config, file)
	return
}
