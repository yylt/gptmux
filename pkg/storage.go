package pkg

import "time"

type Storage interface {
	ListBucket() []Bucket
	CreateBucket(name string, ttl time.Duration) (Bucket, error)
	GetBucket(name string) (Bucket, error)
}

type Bucket interface {
	Set(key string, value []byte, ttl time.Duration) error // 向数据库中设置一个新的键值对，并指定过期时间
	Get(key string) ([]byte, error)                        // 根据键获取存储在数据库中的值
	Delete(key string) error                               // 从数据库中删除指定的键值对
	Iter(func(string, string) bool)
}
