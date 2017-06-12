package gotana

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	URL "net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	EVENT_SCRAPER_OPENED     = "SCRAPER_OPENED"
	EVENT_SCRAPER_CLOSED     = "SCRAPER_CLOSED"
	EVENT_SAVEABLE_EXTRACTED = "SAVEABLE_EXTRACTED"
	STATE_INITIAL            = "INTITIAL"
	STATE_RUNNING            = "RUNNING"
	STATE_STOPPING           = "STOPPING"
	TIMEOUT_DIALER           = time.Duration(time.Second * 30)
	TIMEOUT_REQUEST          = time.Duration(time.Second * 30)
	TIMEOUT_TLS              = time.Duration(time.Second * 10)
)

type SaveableItem interface {
	Scraper() *Scraper
	Validate() bool
	RecordData() []string
}

type ScraperMixin struct {
	Proxy ScrapedItem
}

func (s *ScraperMixin) SetProxy(proxy ScrapedItem) *ScraperMixin {
	s.Proxy = proxy
	return s
}

func (item ScraperMixin) Scraper() *Scraper {
	return item.Proxy.scraper
}

type recordWriter interface {
	Write(record []string) error
	Flush()
}

type ScrapingHandlerFunc func(ScrapedItem, chan<- SaveableItem)

func GetHref(t html.Token) (ok bool, href string) {
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	return
}

type extensionParameters struct {
	scraper *Scraper
	item    SaveableItem
}

type Extension interface {
	ScraperStarted(scraper *Scraper)
	ScraperStopped(scraper *Scraper)
	ItemScraped(scraper *Scraper, item SaveableItem)
}

type Extractable interface {
	Extract(io.ReadCloser, func(string))
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

type ScrapedItem struct {
	Url      string
	Response *http.Response
	scraper  *Scraper
}

func (proxy ScrapedItem) String() (result string) {
	result = fmt.Sprintf("Result of scraping: %s", proxy.Url)
	return
}

func (proxy ScrapedItem) CheckIfRedirected() bool {
	return proxy.Url != proxy.Response.Request.URL.String()
}

func (proxy ScrapedItem) finalResponseBody() (io.ReadCloser, error) {
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

func (proxy ScrapedItem) HTMLDocument() (document *goquery.Document, err error) {
	responseBody, err := proxy.finalResponseBody()

	if err == nil {
		document, err = goquery.NewDocumentFromReader(responseBody)
	}

	return
}

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
	TcpAddress        string
	OutFileName       string
	Meta              *EngineMeta
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

	f, writer := GetWriter(engine)

	if f != nil {
		defer f.Close()
	}

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

			scraper.engine.Meta.IncrSaved(scraper)
			SaveItem(item, writer)
		}
	}
}

func (engine *Engine) startTCPServer() {
	if engine.TcpAddress != "" {
		server := NewTCPServer(engine.TcpAddress, engine)
		server.Start()
	}
}

func (engine *Engine) Start() {
	defer engine.Cleanup()

	engine.state = STATE_RUNNING

	for _, scraper := range engine.scrapers {
		go scraper.Start()
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go engine.startTCPServer()
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

func (engine *Engine) GetScraper(name string) *Scraper {
	for _, scraper := range engine.scrapers {
		if scraper.Name == name {
			return scraper
		}
	}
	return nil
}

func (engine *Engine) PushScraper(scrapers ...*Scraper) *Engine {
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

func (engine *Engine) FromConfig(config *ScraperConfig) *Engine {
	engine.TcpAddress = config.TcpAddress
	engine.OutFileName = config.OutFileName

	for _, configData := range config.Scrapers {
		extractor := defaultExtractor()
		switch configData.Extractor {
		case "link":
			extractor = &LinkExtractor{}
		default:
			break
		}
		scraper := NewScraper(configData.Name, configData.Url, configData.RequestLimit, extractor)
		engine.PushScraper(scraper)
	}

	return engine
}

type Scraper struct {
	crawled      int
	successful   int
	failed       int
	handler      ScrapingHandlerFunc
	fetchMutex   *sync.Mutex
	crawledMutex *sync.Mutex
	Name         string
	Domain       string
	BaseUrl      string
	CurrentUrl   string
	fetchedUrls  map[string]bool
	engine       *Engine
	extractor    Extractable
	chDone       chan struct{}
	chRequestUrl chan string
	requestLimit int
}

func (scraper *Scraper) MarkAsFetched(url string) {
	scraper.fetchMutex.Lock()
	defer scraper.fetchMutex.Unlock()

	scraper.CurrentUrl = url
	scraper.fetchedUrls[url] = true
}

func (scraper *Scraper) CheckIfShouldStop() (ok bool) {
	scraper.crawledMutex.Lock()
	defer scraper.crawledMutex.Unlock()
	stats := scraper.engine.Meta.ScraperStats[scraper.Name]

	if stats.crawled == scraper.engine.limitCrawl {
		Logger().Warningf("Crawl limit exceeded: %s", scraper)
		ok = true
	} else if stats.failed == scraper.engine.limitFail {
		Logger().Warningf("Fail limit exceeeded: %s", scraper)
		ok = true
	} else if stats.failed == 1 && scraper.crawled == 1 {
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
	if strings.Contains(sourceUrl, scraper.Domain) && strings.Index(sourceUrl, "http") == 0 {
		url = sourceUrl
		ok = true
	} else if strings.Index(sourceUrl, "/") == 0 {
		url = scraper.BaseUrl + sourceUrl
		ok = true
	}
	return
}

func (scraper *Scraper) RunExtractor(resp *http.Response) {
	defer SilentRecover("EXTRACTOR")

	scraper.extractor.Extract(resp.Body, func(url string) {
		ok, url := scraper.CheckUrl(url)

		if ok {
			scraper.chRequestUrl <- url
		}
	})
}

func (scraper *Scraper) Stop() {
	Logger().Warningf("Stopping %s", scraper)
	scraper.engine.notifyExtensions(EVENT_SCRAPER_CLOSED,
		extensionParameters{scraper: scraper})

	scraper.chDone <- struct{}{}
	scraper.engine.wg.Done()
}

func (scraper *Scraper) Start() {
	scraper.engine.wg.Add(1)
	Logger().Infof("Starting: %s", scraper)
	scraper.engine.notifyExtensions(EVENT_SCRAPER_OPENED,
		extensionParameters{scraper: scraper})

	scraper.chRequestUrl <- scraper.BaseUrl
	duration := time.Duration(scraper.requestLimit)

	if scraper.requestLimit == 0 {
		duration = defaultRequestLimit()
	}

	limiter := time.Tick(time.Millisecond * duration)

	for {
		select {
		case url := <-scraper.chRequestUrl:
			<-limiter
			go scraper.Fetch(url)
		case <-scraper.chDone:
			Logger().Warningf("Stopped %s", scraper)
			scraper.engine.IncrFinishedCounter()
			scraper.engine.chDone <- struct{}{}
			return
		}
	}
	return
}

func (scraper *Scraper) Notify(url string, resp *http.Response) {
	scraper.engine.Meta.IncrScraped(scraper)
	scraper.engine.chScraped <- NewScrapedItem(url, scraper, resp)
}

func (engine *Engine) PrepareRequest(request *http.Request) *http.Request {
	for _, middleware := range engine.requestMiddleware {
		request = middleware(request)
	}
	return request
}

func (scraper *Scraper) Fetch(url string) (resp *http.Response, err error) {
	if ok := scraper.CheckIfFetched(url); ok {
		return
	}
	scraper.MarkAsFetched(url)

	Logger().Infof("Fetching: %s", url)
	tic := time.Now()

	req, _ := http.NewRequest("GET", url, nil)
	req = scraper.engine.PrepareRequest(req)

	resp, err = NewHTTPClient().Do(req)

	statusCode := 0
	if err == nil {
		statusCode = resp.StatusCode
	}

	Logger().Debugf("[%d]Request to %s took: %s", statusCode, url, time.Since(tic))

	isSuccessful := (err == nil)

	scraper.engine.Meta.UpdateRequestStats(scraper, isSuccessful, req, resp)

	if err == nil {
		scraper.Notify(url, resp)
		scraper.RunExtractor(resp)
	} else {
		Logger().Warningf("Failed to crawl %s", url)
		Logger().Warning(err)
	}

	if scraper.CheckIfShouldStop() {
		scraper.Stop()
	}
	return
}

func (scraper *Scraper) SetHandler(handler ScrapingHandlerFunc) *Scraper {
	scraper.handler = handler
	return scraper
}

func (scraper *Scraper) String() (result string) {
	stats := scraper.engine.Meta.ScraperStats[scraper.Name]
	result = fmt.Sprintf("<Scraper: %s>. Crawled: %d, successful: %d, failed: %d. Items scraped: %d, saved: %d",
		scraper.Domain, stats.crawled, stats.successful, stats.failed, stats.scraped, stats.saved)
	return
}

func NewEngine() (r *Engine) {
	r = &Engine{
		state:      STATE_INITIAL,
		Meta:       NewEngineMeta(),
		limitCrawl: 10000,
		limitFail:  500,
		finished:   0,
		chDone:     make(chan struct{}),
		chScraped:  make(chan ScrapedItem),
		chItems:    make(chan SaveableItem, 10),
	}
	return
}

func NewScraper(name string, sourceUrl string, requestLimit int, extractor Extractable) (s *Scraper) {
	parsed, err := URL.Parse(sourceUrl)
	if err != nil {
		Logger().Infof("Inappropriate URL: %s", sourceUrl)
		return
	}

	if extractor == nil {
		Logger().Warning("Switching to default extractor")
		extractor = defaultExtractor()
	}

	s = &Scraper{
		Name:         name,
		Domain:       parsed.Host,
		BaseUrl:      sourceUrl,
		fetchedUrls:  make(map[string]bool),
		crawledMutex: &sync.Mutex{},
		fetchMutex:   &sync.Mutex{},
		extractor:    extractor,
		chDone:       make(chan struct{}),
		chRequestUrl: make(chan string, 5),
		requestLimit: requestLimit,
	}
	return
}

func NewScrapedItem(url string, scraper *Scraper, resp *http.Response) ScrapedItem {
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	return ScrapedItem{
		Response: resp,
		Url:      url,
		scraper:  scraper,
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

func defaultExtractor() Extractable {
	return &LinkExtractor{}
}

func defaultRequestLimit() time.Duration {
	return time.Duration(1)
}

func GetWriter(engine *Engine) (*os.File, recordWriter) {
	if f, err := os.OpenFile(engine.OutFileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666); err == nil && f != nil {
		switch {
		case strings.HasSuffix(engine.OutFileName, ".csv"):
			Logger().Infof("Using CSV writer.")
			return f, csv.NewWriter(f)
		default:
			Logger().Warningf("Cannot write to: %s. Unsupported extension.", engine.OutFileName)
			return nil, nil
		}

	}
	return nil, nil
}

func SaveItem(item SaveableItem, writer recordWriter) {
	if writer == nil {
		return
	}

	if !item.Validate() {
		Logger().Warning("Item is not valid. Skipping...")
		return
	}

	defer writer.Flush()
	writer.Write(item.RecordData())
}

type ScraperConfig struct {
	Project     string `required:"true"`
	TcpAddress  string
	OutFileName string
	Scrapers    []struct {
		RequestLimit int `required:"true"`
		Extractor    string
		Name         string `required:"true"`
		Url          string `required:"true"`
	}
}

func NewSpiderConfig(file string) (config *ScraperConfig) {
	config = &ScraperConfig{}
	ProcessFile(config, file)
	return
}
