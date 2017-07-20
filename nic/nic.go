package nic

import (
	"time"

	"github.com/zero-os/0-ork/utils"

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
const tbfBuffer = 1600
const tbfLimit = 3000

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

type rate struct {
	bw    uint64
	delay uint32
}

var rates = map[int]rate{
	1:  {0, 0},
	2:  {1.25e+8, 0},     // bw: 1000mbit
	3:  {6.25e+7, 0},     // bw: 500mbit
	4:  {2.5e+7, 0},      // bw: 200mbit
	5:  {1.25e+7, 0},     // bw: 100mbit
	6:  {6.25e+6, 0},     // bw: 50mbit
	7:  {1.25e+6, 10000}, // bw: 10mbit, delay: 10ms
	8:  {250000, 20000},  // bw: 2mbit, delay: 20ms
	9:  {125000, 50000},  // bw: 1mbit, delay: 50ms
	10: {62500, 100000},  // bw: 500kbit, delay: 100ms
	11: {25000, 200},     // bw: 200kbit, delay: 200ms
}

// Delta is a small closure over the counters, returning the delta against previous
// first = initial value
func delta(first uint64) func(uint64) uint64 {
	keep := first
	return func(delta uint64) uint64 {
		v := delta - keep
		keep = delta
		return v
	}
}

func getQdiscHandle(link netlink.Link, qdiscType string, parent uint32) (uint32, error) {
	qdiscList, err := netlink.QdiscList(link)
	if err != nil {
		utils.LogToKernel("ORK: error getting qdisc list for interface with name %v\n", link.Attrs().Name)
		log.Errorf("ORK: error getting qdisc list for interface with name %v: %v", link.Attrs().Name, err)
		return 0, err
	}

	for _, qdisc := range qdiscList {
		if qdisc.Type() == qdiscType && qdisc.Attrs().Parent == parent {
			return qdisc.Attrs().Handle, nil
		}
	}
	err = fmt.Errorf("ORK: failed to find qdisk %v with parent %v", qdiscType, parent)
	log.Error(err)
	return 0, err
}

type Nic struct {
	name     string
	memUsage uint64
	cpuUsage float64
	rates    ifaceRates
	ewma     ifaceEWMA
	netUsage utils.NetworkUsage
	rate     int
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
		log.Errorf("ORK: error getting link for %v: %v", n.name, err)
		return
	}

	err = netlink.LinkSetDown(link)

	if err != nil {
		utils.LogToKernel("ORK: error setting down interface with name %v\n", n.name)
		log.Errorf("ORK: error setting down interface with name %v: %v", n.name, err)
		return
	}
	utils.LogToKernel("ORK: successfully set down interface with name %v\n", n.name)
	log.Debug("Successfully set down interface with name %v", n.name)
	return
}

func (n Nic) applyTbf(link netlink.Link, parent uint32) error {
	//squeezing: tc qdisc add dev $NIC parent 1:1 handle 10: tbf rate 1mbit buffer 1600 limit 3000
	qdiskAttrs := netlink.QdiscAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    parent,
	}
	qdisc := netlink.Tbf{
		QdiscAttrs: qdiskAttrs,
		Rate:       rates[n.rate].bw,
		Buffer:     tbfBuffer,
		Limit:      tbfLimit,
	}

	err := netlink.QdiscAdd(&qdisc)
	if err != nil {
		utils.LogToKernel("ORK: error adding tbf qdisc for interface with name %v\n", n.name)
		log.Errorf("ORK: error adding tbf qdisc for interface with name %v: %v", n.name, err)
		return err
	}
	return nil
}

func (n Nic) applyNetem(link netlink.Link, parent uint32) error {
	//latency: tc qdisc add dev $NIC root handle 1:0 netem delay 200ms
	qdiscAttrs := netlink.QdiscAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    parent,
	}
	netemAttrs := netlink.NetemQdiscAttrs{
		Latency: rates[n.rate].delay,
	}
	qdisc := netlink.NewNetem(qdiscAttrs, netemAttrs)

	err := netlink.QdiscAdd(qdisc)
	if err != nil {
		utils.LogToKernel("ORK: error adding netem qdisc for interface with name %v\n", n.name)
		log.Errorf("ORK: error adding netem qdisc for interface with name %v: %v", n.name, err)
		return err
	}

	return nil
}

func (n Nic) squeeze() {
	n.rate++
	newRate, ok := rates[n.rate]
	// Nic reached maximum rate and needs to be setdown
	if !ok {
		n.setDown()
		return
	}

	link, err := netlink.LinkByName(n.name)
	if err != nil {
		utils.LogToKernel("ORK: error getting link for interface with name %v\n", n.name)
		log.Errorf("ORK: error getting link for %v: %v", n.name, err)
		return
	}

	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		utils.LogToKernel("ORK: error getting qdisc list for interface with name %v\n", link.Attrs().Name)
		log.Errorf("ORK: error getting qdisc list for interface with name %v: %v", link.Attrs().Name, err)
		return
	}

	for _, qdisc := range qdiscs {
		if qdisc.Type() != "noqueue" && qdisc.Attrs().Parent == netlink.HANDLE_ROOT {
			err := netlink.QdiscDel(qdisc)
			if err != nil {
				utils.LogToKernel("ORK: error deleting qdisc %v\n", qdisc)
				log.Errorf("ORK: error deleting qdisc for %v: %v", qdisc, err)
				return
			}
		}

	}
	parent := uint32(netlink.HANDLE_ROOT)

	if newRate.bw > 0 {
		err := n.applyTbf(link, parent)
		if err == nil {
			handle, err := getQdiscHandle(link, "tbf", parent)
			if err == nil {
				parent = handle
			}
		}
	}
	if newRate.delay > 0 {
		n.applyNetem(link, parent)
	}

	return
}

// Kill sets down the nic if it exceeded the network threshold otherwise squeeses it.
func (n Nic) Kill() {
	if n.netUsage.Txb >= threshold ||
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
			nic.rates.rxb = delta(stats.rxb)
			nic.rates.txb = delta(stats.txb)
			nic.rates.rxp = delta(stats.rxp)
			nic.rates.txp = delta(stats.txp)
			nic.ewma.rxb = ewma.NewMovingAverage()
			nic.ewma.txb = ewma.NewMovingAverage()
			nic.ewma.rxp = ewma.NewMovingAverage()
			nic.ewma.txp = ewma.NewMovingAverage()
			nic.netUsage.Rxb = nic.ewma.rxb.Value() / float64(stats.rxb)
			nic.netUsage.Txb = nic.ewma.txb.Value() / float64(stats.txb)
			nic.netUsage.Rxp = nic.ewma.rxp.Value() / float64(stats.rxp)
			nic.netUsage.Txp = nic.ewma.txp.Value() / float64(stats.txp)
			nic.rate = 1

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
		log.Error("ORK: error reading dir /sys/class/net:", err)
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
