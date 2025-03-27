package simulator

import (
	"context"
	"fmt"
	"sync"
	"time"
	"github.com/redis/go-redis/v9"
	"go_distributed_primitives/lock"
)

var (
	ctx      = context.Background()
	stockKey = "product_stock"
	lockKey  = "lock:product"
)

func DecrementStock(client *redis.Client) (bool, error) {
	stock, err := client.Get(ctx, stockKey).Int()
	if err != nil {
		return false, err
	}
	if stock <= 0 {
		return false, nil
	}
	err = client.Decr(ctx, stockKey).Err()
	return err == nil, err
}

func RunSimulationWithDistributedLock(client *redis.Client, buyerCount int) {
	var wg sync.WaitGroup

	client.Set(ctx, stockKey, 10, 0)

	for i := 0; i < buyerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lock := lock.NewRedisLock(
				client,
				lockKey,
				fmt.Sprintf("goroutine-%d", id),
				5*time.Second,
				100*time.Millisecond,
				20,
			)

			if err := lock.TryLockWithRetry(); err != nil {
				fmt.Printf("[Buyer %d] ❌ Failed to acquire lock\n", id)
				return
			}
			defer lock.Unlock()

			ok, err := DecrementStock(client)
			if err != nil {
				fmt.Printf("[Buyer %d] ⚠️ Error checking stock: %v\n", id, err)
				return
			}
			if ok {
				fmt.Printf("[Buyer %d] ✅ Successfully bought the item!\n", id)
			} else {
				fmt.Printf("[Buyer %d] ❌ Sold out\n", id)
			}
		}(i)
	}

	wg.Wait()

	finalStock, _ := client.Get(ctx, stockKey).Result()
	fmt.Println("📦 Final stock:", finalStock)
}

func RunSimulationWithoutLock(client *redis.Client, buyerCount int) {
	var wg sync.WaitGroup
	ctx := context.Background()

	client.Set(ctx, stockKey, 10, 0)

	for i := 0; i < buyerCount; i++ {
		wg.Add(1)
		go func(id int) {
			
			defer wg.Done()

			stock, err := client.Get(ctx, stockKey).Int()
			if err != nil {
				fmt.Printf("[Buyer %d] ❌ Failed to get stock: %v\n", id, err)
				return
			}

			if stock <= 0 {
				fmt.Printf("[Buyer %d] ❌ Sold out\n", id)
				return
			}

			time.Sleep(5 * time.Millisecond)

			err = client.Decr(ctx, stockKey).Err()
			if err != nil {
				fmt.Printf("[Buyer %d] ⚠️ Failed to decrement stock: %v\n", id, err)
			} else {
				fmt.Printf("[Buyer %d] ✅ Bought item (no lock!)\n", id)
			}
		}(i)
	}

	wg.Wait()

	finalStock, _ := client.Get(ctx, stockKey).Result()
	fmt.Println("📦 Final stock (no lock):", finalStock)
}

