package fairusage

import (
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
)

var log = logging.MustGetLogger("ORK")

const threshold  = 0.001
const quarantineTime int64 = 60
const warnTime int64 = 30
const releaseTime int64 = 30


func Monitor(c *cache.Cache) error {
	log.Debug("Monitoring fair usage")

	activities := GetFairUsageActivities(c)

	for _, activity := range activities {
		if activity.CPUAverage() > threshold {
			activity.Limit(warnTime, quarantineTime)
			continue
		}
		activity.UnLimit(releaseTime, threshold)
	}
	return nil
}
