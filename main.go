package main

import (
	"github.com/0-complexity/ORK/cpu"
	"github.com/0-complexity/ORK/domain"
	"github.com/0-complexity/ORK/memory"
	"github.com/0-complexity/ORK/process"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"time"
)

var log = logging.MustGetLogger("ORK")

func monitorMemory(c *cache.Cache) error {
	for {
		if err := memory.Monitor(c); err != nil {
			log.Error(err)
		}
		time.Sleep(time.Second)
	}
}

func monitorCPU(c *cache.Cache) error {
	for {
		if err := cpu.Monitor(c); err != nil {
			log.Error(err)
		}
		time.Sleep(time.Second)
	}
}

func updateCache(c *cache.Cache) error {
	for {

		if err := domain.UpdateCache(c); err != nil {
			log.Error(err)
		}

		if err := process.UpdateCache(c); err != nil {
			log.Error(err)
		}

		time.Sleep(time.Second)
	}
}

func main() {
	c := cache.New(cache.NoExpiration, time.Minute)
	go updateCache(c)
	go monitorMemory(c)
	go monitorCPU(c)
	//wait
	select {}
}
