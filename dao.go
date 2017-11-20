package gotana

import (
	"github.com/go-redis/redis"
)

type DAO interface {
	Write(scraperName string, data []byte) error
	GetLatest(scraperName string) error
	GetAll(scraperName string) error
}

func SaveItem(item SaveableItem, dao DAO) {
	if dao == nil {
		return
	}

	if !item.Validate() {
		Logger().Warning("Item is not valid. Skipping...")
		return
	}

	scraperName := item.Scraper().Name
	if data, err := item.RecordData(); err == nil {
		dao.Write(scraperName, data)
	}
}

type RedisDAO struct {
	client *redis.Client
}

func (r RedisDAO) KeyPrefixed(key string) string {
	return "gotana-" + key
}

func (r RedisDAO) Write(scraperName string, data []byte) error {
	stringData := string(data[:])
	key := r.KeyPrefixed(scraperName)
	r.client.LPush(key, stringData)
	return nil
}

func (r RedisDAO) GetLatest(scraperName string) error {
	return nil
}

func (r RedisDAO) GetAll(scraperName string) error {
	return nil
}

func (r RedisDAO) String() string {
	return "RedisDAO"
}

func NewRedisDao(address string) (dao RedisDAO) {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: "",
		DB:       0,
	})
	dao = RedisDAO{
		client: client,
	}
	return
}
