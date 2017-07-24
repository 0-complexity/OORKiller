package activity

import (
	"github.com/patrickmn/go-cache"
	"github.com/zero-os/0-ork/domain"
	"github.com/zero-os/0-ork/utils"
	"sort"
)

type Activity interface {
	CPU() float64
	Memory() uint64
	Network() utils.NetworkUsage
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

func GetActivitiesSorted(c *cache.Cache, less Less) []Activity {
	activities := GetActivities(c)

	allActivities := Activities{activities, less}
	sort.Sort(allActivities)

	return allActivities.Activities
}

func GetActivities(c *cache.Cache) []Activity {
	items := c.Items()
	activities := make([]Activity, 0, c.ItemCount())

	for _, item := range items {
		activities = append(activities, item.Object.(Activity))
	}

	return activities
}

func EvictActivity(key string, obj interface{}) {
	switch t := obj.(type) {
	case domain.Domain:
		dom := t.GetDomain()
		dom.Free()
	}
}
