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

type Activities struct {
	Activities []CPU
}

func (a Activities) Len() int { return len(a.Activities) }

func (a Activities) Swap(i, j int) {
	a.Activities[i], a.Activities[j] = a.Activities[j], a.Activities[i]
}

func (a Activities) Less(i, j int) bool {
	ai := a.Activities[i]
	aj := a.Activities[j]

	return ai.CPU() > aj.CPU()
}

func GetCPUActivities(c *cache.Cache) []CPU {
	items := c.Items()
	activities := make([]CPU, 0, c.ItemCount())

	for _, item := range items {
		activity, ok := item.Object.(CPU)
		if !ok {
			continue
		}
		activities = append(activities, activity)
	}
	allActivities := Activities{activities}
	sort.Sort(allActivities)

	return allActivities.Activities
}
