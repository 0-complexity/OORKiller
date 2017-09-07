package network

import (
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/0-ork/utils"
)

type Network interface {
	Network() utils.NetworkUsage
	Kill() error
	Name() string
}

func GetNetworkActivities(c *cache.Cache) []Network {
	items := c.Items()
	activities := make([]Network, 0, c.ItemCount())

	for _, item := range items {
		activity, ok := item.Object.(Network)
		if !ok {
			continue
		}
		activities = append(activities, activity)
	}
	return activities
}
