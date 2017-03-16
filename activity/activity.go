package activity

import (
	"github.com/0-complexity/ORK/domain"
	"github.com/patrickmn/go-cache"
	"sort"
)

type Activity interface {
	CPU() float64
	Memory() uint64
	Kill()
	Priority() int
}
type Sorter func([]Activity, int, int) bool

type Activities struct {
	Activities []Activity
	Sort       Sorter
}

func (a Activities) Len() int { return len(a.Activities) }

func (a Activities) Swap(i, j int) {
	a.Activities[i], a.Activities[j] = a.Activities[j], a.Activities[i]
}

func (a Activities) Less(i, j int) bool {
	iP := a.Activities[i].Priority()
	jP := a.Activities[j].Priority()

	if iP != jP {
		return iP > jP
	}

	return a.Sort(a.Activities, i, j)
}

func ActivitiesByMem(a []Activity, i, j int) bool {
	return a[i].Memory() > a[j].Memory()
}

func ActivitiesByCPU(a []Activity, i, j int) bool {
	return a[i].CPU() > a[j].CPU()
}

func GetActivities(c *cache.Cache, sorter Sorter) []Activity {
	items := c.Items()
	activities := make([]Activity, 0, c.ItemCount())

	for _, item := range items {
		activities = append(activities, item.Object.(Activity))
	}

	allActivities := Activities{activities, sorter}
	sort.Sort(allActivities)

	return allActivities.Activities
}

func EvictActivity(key string, obj interface{}) {
	switch t := obj.(type) {
	case domain.Domain:
		dom := t.GetDomain()
		defer dom.Free()
	}
}
