// Package cpu implements cpu monitoring
package cpu

import (
	"github.com/VividCortex/ewma"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	ps_cpu "github.com/shirou/gopsutil/cpu"
	"github.com/zero-os/0-ork/activity"
)

const cpuThreshold float64 = 90.0 // cpuThreshold holds the percentage of cpu consumption at which ork should kill activities
var log = logging.MustGetLogger("ORK")
var cpuEwma = ewma.NewMovingAverage(60)
var killCounter = 0

// isCPUOk returns a true if the CPU consumption is below the defined threshold
func isCPUOk() (bool, error) {
	percent, err := ps_cpu.Percent(0, false)

	if err != nil {
		log.Error("Error getting available memory")
		return false, err
	}
	cpuEwma.Add(percent[0])

	if cpuEwma.Value() < cpuThreshold {
		killCounter = 0
		log.Debugf("CPU consumption is below threshold: %v", cpuEwma.Value())
		return true, nil
	}
	killCounter += 1

	if killCounter >= 5 {
		log.Debugf("CPU consumption is above threshold: %v and kill counter is %v", cpuEwma.Value(), killCounter)
		return false, nil
	}

	log.Debugf("CPU consumption is above threshold: %v and kill counter is %v", cpuEwma.Value(), killCounter)
	return true, nil
}

// Monitor checks the cpu consumption and if it exceeds  cpuThreshold it kills
// activities until the consumption is bellow the threshold.
func Monitor(c *cache.Cache) error {
	log.Debug("Monitoring CPU")

	cpuOk, err := isCPUOk()
	if err != nil {
		return err
	}
	if cpuOk == true {
		return nil
	}

	activities := activity.GetActivitiesSorted(c, activity.ActivitiesByCPU)

	for i := 0; i < len(activities) && cpuOk == false; i++ {
		activ := activities[i]
		if err := activ.Kill(); err == nil {
			c.Delete(activ.Name())
			killCounter = 0
		}
		if cpuOk, err = isCPUOk(); err != nil {
			return err
		}
	}

	return nil
}
