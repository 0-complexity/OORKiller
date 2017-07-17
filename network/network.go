package network

import (
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/ORK/activity"
)

var log = logging.MustGetLogger("ORK")

const networkThreshold float64 = 10000
const warningThreshold float64 = 8000

// Monitor checks the network consumption per interface and if the rate is higher than the threshold, it shutsdown the
// interface exceeding the networkThreshhold
func Monitor(c *cache.Cache) error {
	log.Info("Monitoring network")

	activities := activity.GetActivities(c, activity.ActivitiesByNetwork)

	for _, activ := range activities {
		if warningThreshold <= activ.Network() && activ.Network() < networkThreshold {
			// add nic latency
			continue
		}
		if activ.Network() >= networkThreshold {
			activ.Kill()
		}

	}
	return nil
}
