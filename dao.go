package gotana

import (
	"encoding/json"
	"github.com/go-redis/redis"
)

type genericStruct map[string]interface{}

type DAO interface {
	SaveItem(name string, data []byte) error
	GetItems(name string) []string
	CountItems(name string) int64
	ProcessItem(items string) genericStruct
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
		dao.SaveItem(scraper, data)
	}
}

type RedisDAO struct {
	client *redis.Client
}

func (r RedisDAO) KeyPrefixed(key string) string {
	return "gotana-" + key
}

func (r RedisDAO) SaveItem(name string, data []byte) error {
	stringData := string(data[:])
	key := r.KeyPrefixed(name)
	r.client.SAdd(key, stringData)
	return nil
}

func (r RedisDAO) GetLatestItem(name string) error {
	return nil
}

func (r RedisDAO) GetItems(name string) []string {
	key := r.KeyPrefixed(name)
	return r.client.SMembers(key).Val()
}

func (r RedisDAO) CountItems(name string) int64 {
	key := r.KeyPrefixed(name)
	return r.client.SCard(key).Val()
}

func (r RedisDAO) ProcessItem(item string) genericStruct {
	var data = genericStruct{}
	json.Unmarshal([]byte(item), &data)
	return data
}

func (r RedisDAO) ProcessItems(items []string) []genericStruct {
	result := make([]genericStruct, len(items))

	for index, item := range items {
		result[index] = r.ProcessItem(item)
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
