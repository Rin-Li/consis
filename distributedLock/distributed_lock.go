package distributedlock

import (
	"context"
	"fmt"
	"time"
	"github.com/redis/go-redis/v9"
)


var ctx = context.Background()

type RedisLock struct {
	client *redis.Client
	key  string
	value string
	expire time.Duration
	retry time.Duration
	maxRetries int
}

func NewRedisLock(client *redis.Client, key string, value string, expire time.Duration, retry time.Duration, maxRetris int) *RedisLock {
	return &RedisLock{
		client: client,
		key: key,
		value: value,
		expire: expire,
		retry: retry,
		maxRetries: maxRetris,
	}
}

// Try to Lock
func (r *RedisLock) TryLock() (bool, error) {
	ok, err := r.client.SetNX(ctx, r.key, r.value, r.expire).Result()
	return ok, err
}

// Try to Lock with retry
func (r *RedisLock) TryLockWithRetry() error {
	for i := 0; i < r.maxRetries; i++{
		ok, err := r.TryLock()
		if err != nil{
			return err
		}

		if ok {
			fmt.Printf("Lock acquired: %s\n", r.value)
			return nil
		}
		time.Sleep(r.retry)
	}
	return fmt.Errorf("failed to acquire lock after %d retries", r.maxRetries)
}

//Unlock
func (r *RedisLock) Unlock() error {
	luaScript := `
		if redis.call("GET", KEYS[1]) == ARGV[1] then 
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`
	res, err := r.client.Eval(ctx, luaScript, []string {r.key}, r.value).Result()
	if err != nil{
		return err
	}
	if res == int64(1){
		fmt.Println("Lock release")
		return nil
	}
	return fmt.Errorf("unlock failed: not the lock owner or lock expired")	
}