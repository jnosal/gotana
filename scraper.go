package gotana

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	URL "net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	EVENT_SCRAPER_OPENED     = "SCRAPER_OPENED"
	EVENT_SCRAPER_CLOSED     = "SCRAPER_CLOSED"
	EVENT_SAVEABLE_EXTRACTED = "SAVEABLE_EXTRACTED"
	STATUS_CODE_INITIAL      = 999
	TIMEOUT_DIALER           = time.Duration(time.Second * 30)
	TIMEOUT_REQUEST          = time.Duration(time.Second * 30)
	TIMEOUT_TLS              = time.Duration(time.Second * 10)
)

type SaveableItem interface {
	Scraper() *Scraper
	Validate() bool
	RecordData() ([]byte, error)
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

type ScrapingHandlerFunc func(ScrapedItem, chan<- SaveableItem)

func getHref(t html.Token) (ok bool, href string) {
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	return
}

func trimHash(s string) string {
	if strings.Contains(s, "#") {
		var index int
		for i, str := range s {
			if strconv.QuoteRune(str) == "'#'" {
				index = i
				break
			}
		}
		return s[:index]
	}
	return s
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
			ok, url := getHref(token)
			if !ok {
				continue
			}
			url = trimHash(url)
			callback(url)
		}
	}
}

type ScraperConfig struct {
	Project      string `required:"true"`
	HttpAddress  string
	TcpAddress   string
	RedisAddress string
	Scrapers     []struct {
		RequestLimit int `required:"true"`
		Extractor    string
		Name         string `required:"true"`
		Url          string `required:"true"`
		Patterns     []struct {
			Type    string `required:"true"`
			Pattern string `required:"true"`
		}
	}
}

type ScraperParams struct {
	Name         string
	Url          string
	RequestLimit int
	Extractor    Extractable
}

type ScrapedItem struct {
	Url       string
	FinalUrl  string   `json:"-"`
	scraper   *Scraper `json:"-"`
	BodyBytes []byte   `json:"-"`
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

func (proxy ScrapedItem) CheckURLPatterns() (result bool) {
	patterns := proxy.scraper.urlPatterns
	if len(patterns) == 0 {
		return true
	}

	result = false
	for _, pattern := range patterns {
		if pattern.Validate(proxy.Url) {
			result = true
			break
		}
	}

	return
}

func (proxy ScrapedItem) ScheduleScraperStop() {
	proxy.scraper.Stop()
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
	Scheme       string
	BaseUrl      string
	CurrentUrl   string
	fetchedUrls  map[string]bool
	engine       *Engine
	extractor    Extractable
	chDone       chan struct{}
	chRequestUrl chan string
	requestLimit int
	urlPatterns  []URLPattern
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
		url = fmt.Sprintf("%s://%s%s", scraper.Scheme, scraper.Domain, sourceUrl)
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

	statusCode := STATUS_CODE_INITIAL
	if err == nil {
		statusCode = resp.StatusCode
	}

	if statusCode != http.StatusOK {
		err = errors.New(fmt.Sprintf("%d is not a valid status code", statusCode))
	}

	Logger().Debugf("[%d]Request to %s took: %s", statusCode, url, time.Since(tic))

	isSuccessful := (err == nil)

	scraper.engine.Meta.UpdateRequestStats(scraper, isSuccessful, req, resp)

	if err == nil {
		Logger().Debugf("Succesfully crawled %s.", url)
		scraper.Notify(url, resp)
		scraper.RunExtractor(resp)
	} else {
		Logger().Warningf("Failed to crawl %s. %s", url, err)
	}

	if scraper.CheckIfShouldStop() {
		scraper.Stop()
	}
	return
}

func (scraper *Scraper) AddPatterns(urlPatterns ...URLPattern) *Scraper {
	scraper.urlPatterns = append(scraper.urlPatterns, urlPatterns...)
	return scraper
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

func NewScraper(params ScraperParams) (s *Scraper) {
	parsed, err := URL.Parse(params.Url)
	if err != nil {
		Logger().Infof("Inappropriate URL: %s", params.Url)
		return
	}

	if params.Extractor == nil {
		Logger().Warning("Switching to default extractor")
		params.Extractor = defaultExtractor()
	}

	if params.Name == "" {
		params.Name = defaultScraperName()
	}

	s = &Scraper{
		Name:         params.Name,
		Scheme:       parsed.Scheme,
		Domain:       parsed.Host,
		BaseUrl:      params.Url,
		fetchedUrls:  make(map[string]bool),
		crawledMutex: &sync.Mutex{},
		fetchMutex:   &sync.Mutex{},
		extractor:    params.Extractor,
		chDone:       make(chan struct{}),
		chRequestUrl: make(chan string, 5),
		requestLimit: params.RequestLimit,
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

func defaultScraperName() (name string) {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 5)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	name = string(b)
	return
}

func NewSpiderConfig(file string) (config *ScraperConfig) {
	config = &ScraperConfig{}
	if file != "" {
		ProcessFile(config, file)
	}
	return
}
