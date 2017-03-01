package main

import (
	"github.com/0-complexity/ORK/memory"
	"github.com/op/go-logging"
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

func main() {
	go monitorMemory()

	//wait
	select {}
}
