package cpu

import (
	"github.com/patrickmn/go-cache"
	"sort"
)

type CPU interface {
	CPU() float64
	Kill() error
	Name() string
}

type Activities []CPU

func (a Activities) Len() int { return len(a) }

func (a Activities) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a Activities) Less(i, j int) bool {
	return a[i].CPU() > a[j].CPU()
}

func GetCPUActivities(c *cache.Cache) Activities {
	items := c.Items()
	activities := make(Activities, 0, c.ItemCount())

	for _, item := range items {
		if activity, ok := item.Object.(CPU); ok {
			activities = append(activities, activity)
		}
	}
	sort.Sort(activities)
	return activities
}
