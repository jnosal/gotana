package gotana

import (
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	URL "net/url"
	"strings"
	"sync"
	"time"
)

const (
	EVENT_SCRAPER_OPENED     = "SCRAPER_OPENED"
	EVENT_SCRAPER_CLOSED     = "SCRAPER_CLOSED"
	EVENT_SAVEABLE_EXTRACTED = "SAVEABLE_EXTRACTED"
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

type Extractable interface {
	Extract(io.ReadCloser, func(string))
}

type LinkExtractor struct {
	Extractable
}

func (extractor *LinkExtractor) Extract(r io.ReadCloser, callback func(string)) {
	page := html.NewTokenizer(r)
	defer r.Close()

	for {
		tokenType := page.Next()
		if tokenType == html.ErrorToken {
			return
		}
		token := page.Token()
		if tokenType == html.StartTagToken && token.DataAtom.String() == "a" {
			ok, url := GetHref(token)
			if !ok {
				continue
			}
			callback(url)
		}
	}
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

type ScrapedItem struct {
	Url       string
	FinalUrl  string
	scraper   *Scraper
	BodyBytes []byte
}

func (proxy ScrapedItem) String() (result string) {
	result = fmt.Sprintf("Result of scraping: %s", proxy.Url)
	return
}

func (proxy ScrapedItem) CheckIfRedirected() bool {
	return proxy.Url != proxy.FinalUrl
}

func (proxy ScrapedItem) FinalResponseBody() (io.ReadCloser, error) {
	if proxy.CheckIfRedirected() {
		client := NewHTTPClient()
		response, err := client.Get(proxy.FinalUrl)
		if err != nil {
			return nil, err
		}
		bodyBytes, _ := ioutil.ReadAll(response.Body)
		proxy.BodyBytes = bodyBytes
	}
	return ioutil.NopCloser(bytes.NewBuffer(proxy.BodyBytes)), nil
}

func (proxy ScrapedItem) HTMLDocument() (document *goquery.Document, err error) {
	responseBody, err := proxy.FinalResponseBody()
	if err == nil {
		document, err = goquery.NewDocumentFromReader(responseBody)
	}

	return
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
		BodyBytes: bodyBytes,
		FinalUrl:  resp.Request.URL.String(),
		Url:       url,
		scraper:   scraper,
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

func NewSpiderConfig(file string) (config *ScraperConfig) {
	config = &ScraperConfig{}
	ProcessFile(config, file)
	return
}
