package gotana

import (
	"github.com/go-redis/redis"
)

type recordWriter interface {
	Write(record []string) error
	Flush()
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

type RedisWriter struct {
	client *redis.Client
}

func (r RedisWriter) Write(record []string) error {
	return nil
}

func (r RedisWriter) Flush() {

}

func (r RedisWriter) String() string {
	return "RedisWriter"
}

func NewRedisWriter(address string) (writer RedisWriter) {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: "",
		DB:       0,
	})
	writer = RedisWriter{
		client: client,
	}
	return
}
