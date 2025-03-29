package lock

import (
	"context"
	"fmt"
	"time"
	"github.com/redis/go-redis/v9"
)


var ctx = context.Background()
//Script for redis check if the key is equal, if yes, deleted
// if not return 0, meaning can not release the lock
const Script = `
if redis.call("GET", KEYS[1]) == ARGV[1] then 
	return redis.call("DEL", KEYS[1])
else
	return 0
end
`

type RedisLock struct {
	client *redis.Client
	key  string
	value string
	expire time.Duration
	retry time.Duration
	maxRetries int
}
//Init 
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

// Try to Lock with retry,
//If the key does not exist, the value is set and the lock is acquired
//If already exists, somebody else holding the lock
func (r *RedisLock) TryLockWithRetry() error {
	for i := 0; i < r.maxRetries; i++{
		ok, err := r.TryLock()
		if err != nil{
			return err
		}

		if ok {
			fmt.Printf("lock acquired: %s\n", r.value)
			return nil
		}
		time.Sleep(r.retry)
	}
	return fmt.Errorf("failed to acquire lock after %d retries", r.maxRetries)
}

//Unlock
//Check the current one is the owner,
//If yes, deleted the key and releases the lock
//If not, returns error, meaning someone other has the lock now or the lock has expired.
func (r *RedisLock) Unlock() error {
	res, err := r.client.Eval(ctx, Script, []string {r.key}, r.value).Result()
	if err != nil{
		return err
	}
	if res == int64(1){
		fmt.Println("lock release")
		return nil
	}
	return fmt.Errorf("unlock failed: not the lock owner or lock expired")	
}