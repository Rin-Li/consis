package semaphore

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

type RedisSema struct {
	client *redis.Client 
	key    string
	limit int   // Max concurrent requests
	expire time.Duration
	incrKey string // Increment key for sorting
	ownerKey string // Owener key for sorting
	timeKey string // Time key overtime
	clearLockKey string // Clear lock key
	clearInterval time.Duration // Clear interval
}

func NewRedisSema(client *redis.Client, key string, limit int, expire time.Duration) *RedisSema {
	return &RedisSema{
		client: client,
		key: key,
		limit: limit,
		expire: expire,
		incrKey: key + ":incr",
		ownerKey: key + ":owner",
		timeKey: key + ":time",
		clearLockKey: key + ":clear",
		clearInterval: 5 * time.Second,
	}
}

// Acquire a semaphore
func (s *RedisSema) Acquire(clientID string) (bool, error){
	now := time.Now().Unix()
	//Increse a key
	order, err := s.client.Incr(ctx, s.incrKey).Result()
	if err != nil{
		return false, err
	}
	//Set the order
	pipe := s.client.Pipeline()
	//Order number
	pipe.ZAdd(ctx, s.ownerKey, redis.Z{Score: float64(order), Member: clientID})
	expireTime := now + s.expire.Nanoseconds()
	//Expire time
	pipe.ZAdd(ctx, s.timeKey, redis.Z{Score: float64(expireTime), Member: clientID})
	_, err = pipe.Exec(ctx)
	if err != nil{
		return false, err
	}

	//Clear expired keys
	s.clearExpiredKeys()

	//Check the current rank
	rank, err := s.client.ZRank(ctx, s.ownerKey, clientID).Result()
	if err != nil{
		return false, err
	}
	//If current rank is less than the limit, give the key.
	if rank < int64(s.limit){
		return true, nil
	} else {
		s.Release(clientID)
		return false, nil
	}

}
//Release a semaphore
func (s *RedisSema) Release(clientID string) error{
	pipe := s.client.Pipeline()
	//Not rank
	pipe.ZRem(ctx, s.ownerKey, clientID)
	//Not expire time
	pipe.ZRem(ctx, s.timeKey, clientID)
	_, err := pipe.Exec(ctx)
	return err
}
//Clear expired keys
func (s *RedisSema) clearExpiredKeys() { 
	lockKey := s.clearLockKey
	//Allow only one process to clear expired keys at a time
	ok, err := s.client.SetNX(ctx, lockKey, 1, s.clearInterval).Result()
	//failed to acquire lock
	if err != nil || !ok {
		return 
	}
	now := float64(time.Now().Unix())
	//Get expired keys
	expiredKeys, err := s.client.ZRangeByScore(ctx, s.timeKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%f", now),
	}).Result()

	if err != nil {
		return 
	}
	
	if len(expiredKeys) == 0 {
		return 
	}
	//Delete expired keys
	pipe := s.client.Pipeline()
	for _, key := range expiredKeys{
		pipe.ZRem(ctx, s.ownerKey, key)
		pipe.ZRem(ctx, s.timeKey, key)
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return 
	}
}
//Check the member has the semaphore or not
func (s *RedisSema) IsHolding(clientID string) (bool, error){
	rank, err := s.client.ZRank(ctx, s.ownerKey, clientID).Result()
	if err == redis.Nil{
		return false, nil
	} else if err != nil{
		return false, err
	}
	return int(rank) < s.limit, nil
}