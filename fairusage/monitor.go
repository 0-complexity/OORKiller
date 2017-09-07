package fairusage

import (
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
)

var log = logging.MustGetLogger("ORK")

const threshold  = 0.8
const quarantineTime int64 = 600
const warnTime int64 = 300
const releaseTime int64 = 300


func Monitor(c *cache.Cache) error {
	log.Debug("Monitoring fair usage")

	activities := GetFairUsageActivities(c)

	for _, activity := range activities {
		if activity.CPUAverage() > threshold {
			log.Debug("Activity %v exceeded fair usage threshold", activity.Name())
			activity.Limit(warnTime, quarantineTime)
			continue
		}
		log.Debug("Activity %v is below fair usage threshold", activity.Name())
		activity.UnLimit(releaseTime, threshold)
	}
	return nil
}
