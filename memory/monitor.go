// Package memory implements a memory monitor
package memory

import (
	"github.com/zero-os/0-ork/activity"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/shirou/gopsutil/mem"
)

// memoryThreshold is the value in MB at which ORK should free-up memory
const memoryThreshold uint64 = 100

var log = logging.MustGetLogger("ORK")

// isMemoryOk returns true if the available is above memoryThreshold
// and false otherwise
func isMemoryOk() (bool, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		log.Debug("Error getting available memory")
		return false, err
	}

	if availableMem := v.Available / (1024 * 1024); availableMem > memoryThreshold {
		log.Debug("Memory consumption is below threshold")
		return true, nil
	}

	log.Debug("Memory consumption is above threshold")
	return false, nil
}

// Monitor checks the memory consumption and if the available memory is below memoryThreshold it kills
// activities until available memory is more than
func Monitor(c *cache.Cache) error {
	log.Info("Monitoring memory")

	memOk, err := isMemoryOk()
	if err != nil {
		return err
	}
	if memOk == true {
		return nil
	}

	activities := activity.GetActivities(c, activity.ActivitiesByMem)

	for i := 0; i < len(activities) && memOk == false; i++ {
		activ := activities[i]
		activ.Kill()
		if memOk, err = isMemoryOk(); err != nil {
			return err
		}
	}
	return nil
}
