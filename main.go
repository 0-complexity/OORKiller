package main

import (
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/ORK/activity"
	"github.com/zero-os/ORK/cpu"
	"github.com/zero-os/ORK/domain"
	"github.com/zero-os/ORK/memory"
	"github.com/zero-os/ORK/network"
	"github.com/zero-os/ORK/nic"
	"github.com/zero-os/ORK/process"
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

func monitorNetwork(c *cache.Cache) error {
	for {
		if err := network.Monitor(c); err != nil {
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

		if err := nic.UpdateCache(c); err != nil {
			log.Error(err)
		}
		time.Sleep(time.Second)
	}
}

func main() {
	c := cache.New(cache.NoExpiration, time.Minute)
	c.OnEvicted(activity.EvictActivity)

	go updateCache(c)
	go monitorMemory(c)
	go monitorCPU(c)
	go monitorNetwork(c)

	//wait
	select {}
}
