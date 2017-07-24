package network

import (
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/0-ork/activity"
)

var log = logging.MustGetLogger("ORK")

const networkThreshold float64 = 70.0

// Monitor checks the network consumption per interface and if the rate is higher than the threshold, it shutsdown the
// interface exceeding the networkThreshhold
func Monitor(c *cache.Cache) error {
	log.Debug("Monitoring network")

	activities := activity.GetActivities(c)

	for _, activ := range activities {
		if activ.Network().Txb >= networkThreshold ||
			activ.Network().Txp >= networkThreshold {
			activ.Kill()
		}

	}
	return nil
}
