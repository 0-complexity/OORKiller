package memory

import (
	"sort"

	"github.com/patrickmn/go-cache"
)

type Memory interface {
	Memory() uint64
	Kill() error
	Name() string
}

type Activities []Memory

func (a Activities) Len() int { return len(a) }

func (a Activities) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a Activities) Less(i, j int) bool {
	return a[i].Memory() > a[j].Memory()
}

func GetMemoryActivities(c *cache.Cache) Activities {
	items := c.Items()
	activities := make(Activities, 0, c.ItemCount())

	for _, item := range items {
		if activity, ok := item.Object.(Memory); ok {
			activities = append(activities, activity)
		}
	}
	sort.Sort(activities)
	return activities
}
