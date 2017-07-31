package main

import (
	"os"
	"time"

	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/urfave/cli"
	"github.com/zero-os/0-ork/activity"
	"github.com/zero-os/0-ork/cpu"
	"github.com/zero-os/0-ork/domain"
	"github.com/zero-os/0-ork/memory"
	"github.com/zero-os/0-ork/network"
	"github.com/zero-os/0-ork/nic"
	"github.com/zero-os/0-ork/process"
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
		c.OnEvicted(activity.EvictActivity)

		log.Info("Starting ORK....")
		go updateCache(c)
		go monitorMemory(c)
		go monitorCPU(c)
		go monitorNetwork(c)

		//wait
		select {}
	}

	app.Run(os.Args)
}
