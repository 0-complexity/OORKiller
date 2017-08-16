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

const byteThreshold float64 = 225000000.0 // 90% of 2Gbit in bytes
const packetThreshold float64 = 36000.0   // 90% of 40kpps

const tbfBuffer = 1600
const tbfLimit = 3000

var log = logging.MustGetLogger("ORK")

type ifStat struct {
	rxb, txb, rxp, txp uint64
}

type ifaceDelta struct {
	rxb, txb, rxp, txp func(uint64) uint64
}

type ifaceEwma struct {
	rxb, txb, rxp, txp ewma.MovingAverage
}

type Nic struct {
	name     string
	memUsage uint64
	cpuUsage float64
	netUsage utils.NetworkUsage
	delta    ifaceDelta
	ewma     ifaceEwma
	rate     int
}

type rate struct {
	bw    uint64
	delay uint32
}

var rates = map[int]rate{
	1:  {2.5e+8, 0},      // bw: 2gbit
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

func getQdiscHandle(link netlink.Link, qdiscType string, parent uint32) (uint32, error) {
	qdiscList, err := netlink.QdiscList(link)
	if err != nil {
		log.Errorf("Error getting qdisc list for interface %v: %v", link.Attrs().Name, err)
		return 0, err
	}

	for _, qdisc := range qdiscList {
		if qdisc.Type() == qdiscType && qdisc.Attrs().Parent == parent {
			return qdisc.Attrs().Handle, nil
		}
	}
	err = fmt.Errorf("Failed to find qdisk %v with parent %v", qdiscType, parent)
	log.Error(err)
	return 0, err
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

func (n Nic) Name() string {
	return n.name
}

func (n Nic) setDown() error {
	link, err := netlink.LinkByName(n.name)
	if err != nil {
		log.Errorf("Error getting link for %v: %v", n.name, err)
		return err
	}

	utils.LogToKernel("ORK: attempting to shut down interface %v\n", n.name)
	log.Debugf("Attempting to shut down interface %v", n.name)
	err = netlink.LinkSetDown(link)

	if err != nil {
		utils.LogAction(utils.NicShutdown, n.name, utils.Error)
		utils.LogToKernel("ORK: error shutting down interface %v\n", n.name)
		log.Errorf("Error shuting down interface %v: %v", n.name, err)
		return err
	}
	utils.LogAction(utils.NicShutdown, n.name, utils.Success)
	utils.LogToKernel("ORK: successfully shut down interface %v\n", n.name)
	log.Infof("Successfully shut down interface %v", n.name)
	return nil
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

	utils.LogToKernel("ORK: limiting bandwith of interface %v to %v\n", n.name, rates[n.rate].bw)
	log.Debugf("Limiting bandwith of interface %v to %v", n.name, rates[n.rate].bw)

	err := netlink.QdiscAdd(&qdisc)
	if err != nil {
		utils.LogToKernel("ORK: error limiting bandwith of interface %v to %v\n", n.name, rates[n.rate].bw)
		log.Errorf("Error limiting bandwith of interface %v to %v: %v", n.name, rates[n.rate].bw, err)
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

	utils.LogToKernel("ORK: adding latency %v to interface %v\n", rates[n.rate].delay, n.name)
	log.Debugf("Adding latency %v to interface %v", rates[n.rate].delay, n.name)

	err := netlink.QdiscAdd(qdisc)
	if err != nil {
		utils.LogToKernel("ORK: error adding latency %v to interface %v\n", rates[n.rate].delay, n.name)
		log.Errorf("Error adding latency %v to interface %v: %v", rates[n.rate].delay, n.name, err)
		return err
	}

	return nil
}

func (n Nic) squeeze() error {
	n.rate++
	newRate, ok := rates[n.rate]
	// Nic reached maximum rate and needs to be setdown
	if !ok {
		return n.setDown()
	}

	link, err := netlink.LinkByName(n.name)
	if err != nil {
		log.Errorf("Error getting link for %v: %v", n.name, err)
		return err
	}

	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		log.Errorf("Error getting qdisc list for interface %v: %v", link.Attrs().Name, err)
		return err
	}

	for _, qdisc := range qdiscs {
		if qdisc.Type() != "noqueue" && qdisc.Attrs().Parent == netlink.HANDLE_ROOT {
			utils.LogToKernel("ORK: Attempting to delete qdisc %v\n", qdisc)
			log.Debugf("Attempting to delete qdisc %v: %v", qdisc, err)

			err := netlink.QdiscDel(qdisc)
			if err != nil {
				utils.LogToKernel("ORK: error deleting qdisc %v\n", qdisc)
				log.Errorf("Error deleting qdisc for %v: %v", qdisc, err)
				return err
			}
			utils.LogToKernel("ORK: successfully deleted qdisc %v\n", qdisc)
			log.Debugf("Successfully deleted qdisc %v: %v", qdisc, err)
		}
		return nil
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
		err := n.applyNetem(link, parent)
		if err != nil {
			return err
		}
	}

	return nil
}

// Kill sets down the nic if it exceeded the network threshold otherwise squeeses it.
func (n Nic) Kill() error {
	if n.netUsage.Txb >= byteThreshold ||
		n.netUsage.Txp >= packetThreshold {
		return n.setDown()
	}
	return n.squeeze()
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
			nic.delta.rxb = utils.Delta(stats.rxb)
			nic.delta.txb = utils.Delta(stats.txb)
			nic.delta.rxp = utils.Delta(stats.rxp)
			nic.delta.txp = utils.Delta(stats.txp)
			nic.ewma.rxb = ewma.NewMovingAverage(180)
			nic.ewma.txb = ewma.NewMovingAverage(180)
			nic.ewma.rxp = ewma.NewMovingAverage(180)
			nic.ewma.txp = ewma.NewMovingAverage(180)
			nic.netUsage.Rxb = nic.ewma.rxb.Value()
			nic.netUsage.Txb = nic.ewma.txb.Value()
			nic.netUsage.Rxp = nic.ewma.rxp.Value()
			nic.netUsage.Txp = nic.ewma.txp.Value()
			nic.rate = 1

			c.Set(iface, nic, time.Minute)
			continue
		}

		// If the nic exists in the cache, add the new statistics to emwa and calculate the new usage percentage.
		nic := n.(Nic)
		nic.ewma.rxb.Add(float64(nic.delta.rxb(stats.rxb)))
		nic.ewma.txb.Add(float64(nic.delta.txb(stats.txb)))
		nic.ewma.rxp.Add(float64(nic.delta.rxp(stats.rxp)))
		nic.ewma.txp.Add(float64(nic.delta.txp(stats.txp)))
		nic.netUsage.Rxb = nic.ewma.rxb.Value()
		nic.netUsage.Txb = nic.ewma.txb.Value()
		nic.netUsage.Rxp = nic.ewma.rxp.Value()
		nic.netUsage.Txp = nic.ewma.txp.Value()

		c.Set(iface, nic, time.Minute)
	}

	return nil
}

func listNics() ([]string, error) {
	var ifaces []string
	l, err := ioutil.ReadDir("/sys/class/net")
	if err != nil {
		log.Errorf("Error reading dir /sys/class/net: %v", err)
		return nil, err
	}
	for _, iface := range l {
		link, err := netlink.LinkByName(iface.Name())
		if err != nil {
			log.Errorf("Error getting link for %v: %v", iface.Name(), err)
			return nil, err
		}

		if link.Type() == "vxlan" {
			ifaces = append(ifaces, iface.Name())
		}
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
	digit, err := strconv.ParseUint(val[0], 10, 64)
	if err != nil {
		log.Errorf("Error parsing int %v", err)
		return 0, err
	}
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
