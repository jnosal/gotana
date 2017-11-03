package gotana

import (
	"github.com/go-redis/redis"
)

type DAO interface {
	Write(scraperName string, data []byte) error
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
	r.client.LPush(r.KeyPrefixed(scraperName), stringData)
	return nil
}

func (r RedisDAO) String() string {
	return "RedisWriter"
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
