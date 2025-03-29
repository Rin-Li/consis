package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

var leakyBucketScript = `
local key = KEYS[1]
local rate = tonumber(ARGV[1])
local capacity = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

-- Give the value key, water, last to bucket
local bucket = redis.call('HMGET', key, 'water', 'last')

local water = tonumber(bucket[1]) or 0
local last_time = tonumber(bucket[2]) or now

-- Calculate how much water should leak between last time and now, and current water in the bucket
local leaked = (now - last_time) / 1000 * rate
water = math.max(0, water - leaked)

if water + 1 > capacity then
	return 0
else
	water = water + 1

	-- record this time
	redis.call('HMSET', key, 'water', water, 'last', now)
	-- Expire
	redis.call('PEXPIRE', key, 60000)
	return 1
end
`

type RedisLeakyBucketLimiter struct{
	client *redis.Client
	key string     
	rate float64   //Rate of Bucket
	capacity float64 // Capacity of Bucket
}

func NewRedisLeakyBucketLimiter(client *redis.Client, key string, rate float64, capacity float64) *RedisLeakyBucketLimiter{
	return &RedisLeakyBucketLimiter{
		client: client,
		key: key,
		rate: rate,
		capacity: capacity,
	}
}

//While allow - can pass, false - can not pass
func(l *RedisLeakyBucketLimiter) Allow() (bool, error){
	now := time.Now().UnixNano()
	ok, err := l.client.Eval(ctx, leakyBucketScript, []string{l.key}, l.rate, l.capacity, now).Int()
	if err != nil {
		return false, err
	}
	return ok == 1, nil
}