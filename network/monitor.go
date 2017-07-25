package network

import (
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/0-ork/activity"
)

var log = logging.MustGetLogger("ORK")

const byteThreshold float64 = 175000000.0 // 70% of 2Gbit in bytes
const packetThreshold float64 = 14000.0 // 70% of 20kpps

// Monitor checks the network consumption per interface and if the rate is higher than the threshold, it shutsdown the
// interface exceeding the networkThreshhold
func Monitor(c *cache.Cache) error {
	log.Debug("Monitoring network")

	activities := activity.GetActivities(c)

	for _, activ := range activities {
		netUsage:= activ.Network()
		if netUsage.Txb >= byteThreshold ||
			netUsage.Txp >= packetThreshold {
			activ.Kill()
		}

	}
	return nil
}
