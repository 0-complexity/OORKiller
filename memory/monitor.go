// Package memory implements a memory monitor
package memory

import (
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/shirou/gopsutil/mem"
	"github.com/zero-os/0-ork/activity"
)

// memoryThreshold is the value in MB at which ORK should free-up memory
const memoryThreshold uint64 = 100
var killCounter = 0

var log = logging.MustGetLogger("ORK")

// isMemoryOk returns true if the available is above memoryThreshold
// and false otherwise
func isMemoryOk() (bool, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		log.Error("Error getting available memory")
		return false, err
	}
	availableMem := v.Available / (1024 * 1024)
	if availableMem > memoryThreshold {
		killCounter = 0
		log.Debugf("Memory available is higher than threshold: %v", availableMem)
		return true, nil
	}
	killCounter += 1

	if killCounter >= 5 {
		log.Debugf("Memory available is lower than threshold: %v and kill counter is %v", availableMem, killCounter)
		return false, nil
	}

	log.Debugf("Memory available is lower than threshold: %v and kill counter is %v", availableMem, killCounter)
	return true, nil

}

// Monitor checks the memory consumption and if the available memory is below memoryThreshold it kills
// activities until available memory is more than
func Monitor(c *cache.Cache) error {
	log.Debug("Monitoring memory")

	memOk, err := isMemoryOk()
	if err != nil {
		return err
	}
	if memOk == true {
		return nil
	}

	activities := activity.GetActivitiesSorted(c, activity.ActivitiesByMem)

	for i := 0; i < len(activities) && memOk == false; i++ {
		activ := activities[i]
		if err = activ.Kill(); err == nil {
			c.Delete(activ.Name())
			killCounter = 0
		}
		if memOk, err = isMemoryOk(); err != nil {
			return err
		}
	}
	return nil
}
