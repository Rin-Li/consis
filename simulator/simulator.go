package simulator

import (
	"context"
	"fmt"
	"go_distributed_primitives/lock"
	"go_distributed_primitives/semaphore"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ctx      = context.Background()
	stockKey = "product_stock"
	lockKey  = "lock:product"
	semKey   = "library_semaphore"
	resourceLimit = 10
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

func simulateLibrary(id int){
	fmt.Printf("🧑‍🎓 Student %d is reading books ...\n", id)
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("🧑‍🎓 Student %d finished reading books\n", id)
}

func RunSimulationWithSemaphore(client *redis.Client, visitorCount int){
	sema := semaphore.NewRedisSema(client, semKey, resourceLimit, 5*time.Second)
	var wg sync.WaitGroup

	var mu sync.Mutex
	currentVisitors := 0
	maxInside := 0

	for i := 0; i < visitorCount; i++{
		wg.Add(1)
		go func (id int){
			defer wg.Done()
			ok, err := sema.Acquire(fmt.Sprintf("student-%d", id))
			if err != nil{
				fmt.Printf(("[Student %d] ❌ Failed to acquire semaphore: %v\n"), id, err)
				return
			}

			if !ok {
				fmt.Printf("[Student %d] ❌ Library is full of people 🥹\n", id)
				return
			}

			mu.Lock()
			currentVisitors++
			if currentVisitors > maxInside {
				maxInside = currentVisitors
			}
			mu.Unlock()

			defer sema.Release(fmt.Sprintf("student-%d", id))
			simulateLibrary(id)
		}(i)
	}
	wg.Wait()
	fmt.Printf("📈 Max inside at once:   %d (Limit: %d)\n", maxInside, resourceLimit)
}

func RunSimulationWithoutSemaphore(client *redis.Client, visitorCount int){
	var wg sync.WaitGroup
	for i := 0; i < visitorCount; i++{
		wg.Add(1)
		go func (id int){
			defer wg.Done()
			simulateLibrary(id)
		}(i)
	}
	wg.Wait()
}