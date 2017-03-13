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

func monitorMemory() error {
	for {
		err := memory.Monitor()
		if err != nil {
			log.Error(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func monitorCPU(c *cache.Cache) error {
	for {
		err := cpu.Monitor(c)
		if err != nil {
			log.Error(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func setDomainsCPUTime(c *cache.Cache) error {
	for {
		err := domain.SetDomainCPUTime(c)
		if err != nil {
			log.Error(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func setProcessesCPUTime(c *cache.Cache) error {
	for {
		err := process.SetProcessCPUUsage(c)
		if err != nil {
			log.Error(err)
		}
		time.Sleep(1 * time.Second)
	}
}

func main() {
	c := cache.New(cache.NoExpiration, 0)
	go setDomainsCPUTime(c)
	go setProcessesCPUTime(c)
	go monitorMemory()
	go monitorCPU(c)
	//wait
	select {}
}
