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

type Activities struct {
	Activities []Memory
}

func (a Activities) Len() int { return len(a.Activities) }

func (a Activities) Swap(i, j int) {
	a.Activities[i], a.Activities[j] = a.Activities[j], a.Activities[i]
}

func (a Activities) Less(i, j int) bool {
	ai := a.Activities[i]
	aj := a.Activities[j]

	return ai.Memory() > aj.Memory()
}

func GetMemoryActivities(c *cache.Cache) []Memory {
	items := c.Items()
	activities := make([]Memory, 0, c.ItemCount())

	for _, item := range items {
		activity, ok := item.Object.(Memory)
		if !ok {
			continue
		}
		activities = append(activities, activity)
	}
	allActivities := Activities{activities}
	sort.Sort(allActivities)

	return allActivities.Activities
}
