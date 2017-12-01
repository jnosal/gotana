package gotana

import (
	fury "github.com/jnosal/gofury"
	"net/http"
	"strconv"
	"strings"
)

type HealthCheckResource struct {
	engine *Engine
}

func (resource HealthCheckResource) Get(meta *fury.Meta) {
	result := map[string]string{"status": "OK"}
	meta.Json(http.StatusOK, result)
}

type StatsResource struct {
	engine *Engine
}

func (resource StatsResource) Get(meta *fury.Meta) {
	result := map[string]interface{}{}

	for _, scraper := range resource.engine.scrapers {
		info := map[string]interface{}{}
		stats := resource.engine.Meta.ScraperStats[scraper.Name]
		info["currentUrl"] = scraper.CurrentUrl
		info["domain"] = scraper.Domain
		info["crawled"] = stats.crawled
		info["succesful"] = stats.successful
		info["scraped"] = stats.scraped
		info["saved"] = stats.saved
		result[scraper.Name] = info
	}

	meta.Json(http.StatusOK, result)
}

type ListByScraperResource struct {
	engine *Engine
}

func (resource ListByScraperResource) Get(meta *fury.Meta) {
	result := map[string]interface{}{}
	name := meta.Query().Get("scraper")
	scraper := resource.engine.GetScraper(name)

	if scraper == nil {
		result["error"] = "Scraper is not defined."
		meta.Json(http.StatusBadRequest, result)
		return
	}

	dao := GetDAO(resource.engine)
	items := dao.GetItems(scraper.Name)

	result["items"] = dao.ProcessItems(items)
	result["count"] = dao.CountItems(scraper.Name)

	meta.Json(http.StatusOK, result)
}

func NewHTTPServer(address string, engine *Engine) (server *fury.Fury) {
	chunks := strings.Split(address, ":")
	host := chunks[0]
	port, _ := strconv.Atoi(chunks[1])
	server = fury.New(host, port)

	server.Route("/api/healthcheck", &HealthCheckResource{engine})
	server.Route("/api/items", &ListByScraperResource{engine})
	server.Route("/api/stats", &StatsResource{engine})
	return
}
