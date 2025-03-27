package main

import (
	"github.com/redis/go-redis/v9"
	"go_distributed_primitives/simulator"
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
    // Run simulation, the number represents the number of buyers
	simulator.RunSimulationWithoutLock(client, 50)
}