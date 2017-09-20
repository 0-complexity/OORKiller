package main

import (
	"os"
	"time"

	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/urfave/cli"
	"github.com/zero-os/0-ork/cpu"
	"github.com/zero-os/0-ork/domain"
	"github.com/zero-os/0-ork/fairusage"
	"github.com/zero-os/0-ork/memory"
	"github.com/zero-os/0-ork/network"
	"github.com/zero-os/0-ork/nic"
	"github.com/zero-os/0-ork/process"
	"github.com/zero-os/0-ork/utils"
)

var log = logging.MustGetLogger("ORK")

func monitorMemory(c *cache.Cache) {
	for {
		if err := memory.Monitor(c); err != nil {
			log.Error(err)
		}
		time.Sleep(time.Second)
	}
}

func monitorCPU(c *cache.Cache) {
	for {
		if err := cpu.Monitor(c); err != nil {
			log.Error(err)
		}
		time.Sleep(time.Second)
	}
}

func monitorNetwork(c *cache.Cache) {
	for {
		if err := network.Monitor(c); err != nil {
			log.Error(err)
		}
		time.Sleep(time.Second)
	}
}

func monitorFairUsage(c *cache.Cache) {
	for {
		if err := fairusage.Monitor(c); err != nil {
			log.Error(err)
		}
		time.Sleep(time.Second)
	}
}

func updateCache(c *cache.Cache) {
	for {
		domain.UpdateCache(c)
		process.UpdateCache(c)
		nic.UpdateCache(c)

		time.Sleep(time.Second)
	}
}

func main() {
	// Disable ork if development is in the kernel parameters
	if utils.Development() {
		select {}
	}

	app := cli.NewApp()
	app.Version = "0.1.0"
	app.Name = "ORK"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "level",
			Value: "INFO",
			Usage: "log level",
		},
	}
	app.Action = func(context *cli.Context) {
		level, err := logging.LogLevel(context.String("level"))
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		backend := logging.NewLogBackend(os.Stdout, "", 0)
		backendLeveled := logging.AddModuleLevel(backend)
		backendLeveled.SetLevel(level, "")
		logging.SetBackend(backendLeveled)

		c := cache.New(cache.NoExpiration, time.Minute)

		log.Info("Starting ORK....")
		go updateCache(c)

		if utils.MonitorCPU() {
			go monitorCPU(c)
		}
		if utils.MonitorMem() {
			go monitorMemory(c)
		}
		if utils.MonitorNetwork() {
			go monitorNetwork(c)
		}
		if utils.MonitorFairUsage() {
			go monitorFairUsage(c)
		}

		//wait
		select {}
	}

	app.Run(os.Args)
}
