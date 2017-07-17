package activity

import (
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/ORK/domain"
	"sort"
)

type Activity interface {
	CPU() float64
	Memory() uint64
	Network() float64
	Kill()
	Priority() int
}
type Less func(Activity, Activity) bool

type Activities struct {
	Activities []Activity
	Sorter     Less
}

func (a Activities) Len() int { return len(a.Activities) }

func (a Activities) Swap(i, j int) {
	a.Activities[i], a.Activities[j] = a.Activities[j], a.Activities[i]
}

func (a Activities) Less(i, j int) bool {
	ai := a.Activities[i]
	aj := a.Activities[j]
	iP := ai.Priority()
	jP := aj.Priority()

	if iP != jP {
		return iP > jP
	}

	return a.Sorter(ai, aj)
}

func ActivitiesByMem(ai, aj Activity) bool {
	return ai.Memory() > aj.Memory()
}

func ActivitiesByCPU(ai, aj Activity) bool {
	return ai.CPU() > aj.CPU()
}

func ActivitiesByNetwork(ai, aj Activity) bool {
	return ai.Network() > aj.Network()
}

func GetActivities(c *cache.Cache, less Less) []Activity {
	items := c.Items()
	activities := make([]Activity, 0, c.ItemCount())

	for _, item := range items {
		activities = append(activities, item.Object.(Activity))
	}

	allActivities := Activities{activities, less}
	sort.Sort(allActivities)

	return allActivities.Activities
}

func EvictActivity(key string, obj interface{}) {
	switch t := obj.(type) {
	case domain.Domain:
		dom := t.GetDomain()
		dom.Free()
	}
}
