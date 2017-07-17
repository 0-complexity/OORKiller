package nic

import (
	"time"

	"github.com/shirou/gopsutil/net"
	"github.com/zero-os/ORK/utils"

	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/vishvananda/netlink"
)

var log = logging.MustGetLogger("ORK")

type Nic struct {
	name        string
	memUsage    uint64
	cpuUsage    float64
	currentNet  uint64
	oldNet      uint64
	currentTime time.Time
	oldTime     time.Time
	netUsage    float64
}

func (n Nic) CPU() float64 {
	return n.cpuUsage
}

func (n Nic) Memory() uint64 {
	return n.memUsage
}

func (n Nic) Network() float64 {
	return n.netUsage
}

func (n Nic) Priority() int {
	return 50
}

func (n Nic) Kill() {
	link, err := netlink.LinkByName(n.name)
	if err != nil {
		utils.LogToKernel("ORK: error getting link for interface with name %v\n", n.name)
		log.Errorf("Error getting link for %v", n.name)
	}

	utils.LogToKernel("ORK: attempting to set down interface with name %v\n", n.name)
	err = netlink.LinkSetDown(link)

	if err != nil {
		utils.LogToKernel("ORK: error setting down interface with name %v\n", n.name)
		log.Errorf("Error setting down interface with name %v: %v", n.name, err)
		return
	}
	utils.LogToKernel("ORK: successfully set down interface with name %v\n", n.name)
	log.Info("Successfully set down interface with name %v", n.name)
	return
}

func UpdateCache(c *cache.Cache) error {
	counters, err := net.IOCounters(true)
	if err != nil {
		log.Error("Error getting nics counters")
	}
	for _, counter := range counters {
		n, ok := c.Get(counter.Name)
		if !ok {
			nic := Nic{
				name:        counter.Name,
				currentNet:  counter.PacketsRecv + counter.PacketsSent,
				currentTime: time.Now(),
			}
			c.Set(counter.Name, nic, time.Minute)
			continue
		}

		nic := n.(Nic)
		nic.oldNet = nic.currentNet
		nic.oldTime = nic.currentTime
		nic.currentNet = counter.PacketsRecv + counter.PacketsSent
		nic.currentTime = time.Now()
		nic.netUsage = float64((nic.currentNet - nic.oldNet)) / (nic.currentTime.Sub(nic.oldTime).Seconds())
		c.Set(counter.Name, nic, time.Minute)
	}

	return nil
}
