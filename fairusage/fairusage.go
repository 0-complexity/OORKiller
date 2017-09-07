package fairusage

import "github.com/patrickmn/go-cache"

type FairUsage interface {
	CPUAverage() float64
	Limit(int64, int64)
	UnLimit(int64, float64)
	Name() string
}

func GetFairUsageActivities(c *cache.Cache) []FairUsage {
	items := c.Items()
	activities := make([]FairUsage, 0, c.ItemCount())

	for _, item := range items {
		activity, ok := item.Object.(FairUsage)
		if !ok {
			continue
		}
		activities = append(activities, activity)
	}

	return activities
}
