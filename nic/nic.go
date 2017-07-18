package nic

import (
	"time"

	"github.com/zero-os/ORK/utils"

	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/VividCortex/ewma"
	"github.com/op/go-logging"
	"github.com/patrickmn/go-cache"
	"github.com/vishvananda/netlink"
)

const threshold float64 = 90.0

var log = logging.MustGetLogger("ORK")

type ifStat struct {
	rxb, txb, rxp, txp uint64
}

type ifaceRates struct {
	rxb, txb, rxp, txp func(uint64) uint64
}

type ifaceEWMA struct {
	rxb, txb, rxp, txp ewma.MovingAverage
}

// Delta is a small closure over the counters, returning the delta against previous
// first = initial value
func Delta(first uint64) func(uint64) uint64 {
	keep := first
	return func(delta uint64) uint64 {
		v := delta - keep
		keep = delta
		return v
	}
}

type Nic struct {
	name     string
	memUsage uint64
	cpuUsage float64
	rates    ifaceRates
	ewma     ifaceEWMA
	netUsage utils.NetworkUsage
}

func (n Nic) CPU() float64 {
	return n.cpuUsage
}

func (n Nic) Memory() uint64 {
	return n.memUsage
}

func (n Nic) Network() utils.NetworkUsage {
	return n.netUsage
}

func (n Nic) Priority() int {
	return 50
}

func (n Nic) setDown() {
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

func (n Nic) squeeze() {
	// @TODO Implement
	return
}

// Kill sets down the nic if it exceeded the network threshold otherwise squeeses it.
func (n Nic) Kill() {
	if n.netUsage.Rxb >= threshold ||
		n.netUsage.Txb >= threshold ||
		n.netUsage.Rxp >= threshold ||
		n.netUsage.Txp >= threshold {
		n.setDown()
		return
	}
	n.squeeze()
	return
}

func UpdateCache(c *cache.Cache) error {
	ifaces, err := listNics()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		stats, err := readStatistics(iface)
		if err != nil {
			continue
		}
		n, ok := c.Get(iface)
		// If the nic doesn't exist in the cache, create a new instance for it
		if !ok {
			var nic Nic
			nic.name = iface
			nic.rates.rxb = Delta(stats.rxb)
			nic.rates.txb = Delta(stats.txb)
			nic.rates.rxp = Delta(stats.rxp)
			nic.rates.txp = Delta(stats.txp)
			nic.ewma.rxb = ewma.NewMovingAverage()
			nic.ewma.txb = ewma.NewMovingAverage()
			nic.ewma.rxp = ewma.NewMovingAverage()
			nic.ewma.txp = ewma.NewMovingAverage()
			nic.netUsage.Rxb = nic.ewma.rxb.Value() / float64(stats.rxb)
			nic.netUsage.Txb = nic.ewma.txb.Value() / float64(stats.txb)
			nic.netUsage.Rxp = nic.ewma.rxp.Value() / float64(stats.rxp)
			nic.netUsage.Txp = nic.ewma.txp.Value() / float64(stats.txp)

			c.Set(iface, nic, time.Minute)
			continue
		}

		// If the nic exists in the cache, add the new statistics to emwa and calculate the new usage percentage.
		nic := n.(Nic)
		nic.ewma.rxb.Add(float64(nic.rates.rxb(stats.rxb)))
		nic.ewma.txb.Add(float64(nic.rates.txb(stats.txb)))
		nic.ewma.rxp.Add(float64(nic.rates.rxp(stats.rxp)))
		nic.ewma.txp.Add(float64(nic.rates.txp(stats.txp)))
		nic.netUsage.Rxb = nic.ewma.rxb.Value() / float64(stats.rxb)
		nic.netUsage.Txb = nic.ewma.txb.Value() / float64(stats.txb)
		nic.netUsage.Rxp = nic.ewma.rxp.Value() / float64(stats.rxp)
		nic.netUsage.Txp = nic.ewma.txp.Value() / float64(stats.txp)

		c.Set(iface, nic, time.Minute)
	}

	return nil
}

func listNics() ([]string, error) {
	var ifaces []string
	l, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		log.Error("Error reading dir /sys/class/net:", err)
		return nil, err
	}
	for _, iface := range l {
		ifaces = append(ifaces, iface.Name())
	}
	return ifaces, nil
}

func readVal(path string) (uint64, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorf("Error reading file %v:%v", path, err)
		return 0, err
	}
	val := strings.Split(string(contents), "\n")
	digit, _ := strconv.ParseUint(val[0], 10, 64)
	return digit, nil
}

func readStatistics(i string) (ifStat, error) {
	v := ifStat{}
	var err error
	path := "/sys/class/net/%v/statistics/%v"

	v.rxb, err = readVal(fmt.Sprintf(path, i, "rx_bytes"))
	if err != nil {
		return ifStat{}, err
	}
	v.txb, err = readVal(fmt.Sprintf(path, i, "tx_bytes"))
	if err != nil {
		return ifStat{}, err
	}
	v.rxp, err = readVal(fmt.Sprintf(path, i, "rx_packets"))
	if err != nil {
		return ifStat{}, err
	}
	v.txp, err = readVal(fmt.Sprintf(path, i, "tx_packets"))
	if err != nil {
		return ifStat{}, err
	}
	return v, nil
}
