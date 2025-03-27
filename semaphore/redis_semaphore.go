package semaphore

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

type RedisSema struct {
	client *redis.Client 
	key    string
	limit int
	exprire int
	expire time.Duration
}
