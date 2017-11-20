package gotana

import (
	"encoding/json"
	"github.com/go-redis/redis"
)

type genericStruct map[string]interface{}

type DAO interface {
	Write(scraper string, data []byte) error
	GetLatestItem(scraper string) error
	GetItems(scraper string, start int64, stop int64) []string
	ProcessItems(items []string) []genericStruct
}

func SaveItem(item SaveableItem, dao DAO) {
	if dao == nil {
		return
	}

	if !item.Validate() {
		Logger().Warning("Item is not valid. Skipping...")
		return
	}

	scraper := item.Scraper().Name
	if data, err := item.RecordData(); err == nil {
		dao.Write(scraper, data)
	}
}

type RedisDAO struct {
	client *redis.Client
}

func (r RedisDAO) KeyPrefixed(key string) string {
	return "gotana-" + key
}

func (r RedisDAO) Write(scraper string, data []byte) error {
	stringData := string(data[:])
	key := r.KeyPrefixed(scraper)
	r.client.LPush(key, stringData)
	return nil
}

func (r RedisDAO) GetLatestItem(scraper string) error {
	return nil
}

func (r RedisDAO) GetItems(scraper string, start int64, stop int64) []string {
	key := r.KeyPrefixed(scraper)
	return r.client.LRange(key, start, stop).Val()
}

func (r RedisDAO) ProcessItems(items []string) []genericStruct {
	result := make([]genericStruct, len(items))

	for index, item := range items {
		var data = genericStruct{}
		json.Unmarshal([]byte(item), &data)
		result[index] = data
	}

	return result
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
