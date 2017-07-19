// Package cpu implements cpu monitoring
package cpu

import (
	"github.com/zero-os/0-ork/activity"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	ps_cpu "github.com/shirou/gopsutil/cpu"
)

const cpuThreshold float64 = 90.0 // cpuThreshold holds the value of the memory threshold
var log = logging.MustGetLogger("ORK")

// isCPUOk returns a true if the CPU consumption is below the defined threshold
func isCPUOk() (bool, error) {
	percent, err := ps_cpu.Percent(0, false)

	if err != nil {
		log.Error("Error getting available memory")
		return false, err
	}

	if percent[0] < cpuThreshold {
		log.Debug("CPU consumption is below threshold: ", percent[0])
		return true, nil
	}

	log.Debug("CPU consumption is above threshold:", percent[0])
	return false, nil
}

// Monitor checks the cpu consumption and if it exceeds  cpuThreshold it kills
// activities until the consumption is bellow the threshold.
func Monitor(c *cache.Cache) error {
	log.Info("Monitoring CPU")

	cpuOk, err := isCPUOk()
	if err != nil {
		return err
	}
	if cpuOk == true {
		return nil
	}

	activities := activity.GetActivities(c, activity.ActivitiesByCPU)

	for i := 0; i < len(activities) && cpuOk == false; i++ {
		activ := activities[i]
		activ.Kill()
		if cpuOk, err = isCPUOk(); err != nil {
			return err
		}
	}

	return nil
}
