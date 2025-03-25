package main

import (
	"github.com/redis/go-redis/v9"
	"consis/simulator"
)

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	simulator.RunSimulationWithoutLock(client, 50)
}