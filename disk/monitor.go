package disk

import (
	"strings"

	"github.com/0-complexity/ORK/utils"
	"github.com/op/go-logging"
	"github.com/shirou/gopsutil/disk"
)

type Threshold struct {
	disk  uint64
	inode uint64
}

type Deletable struct {
	path       string
	deleteFunc func(string)
}

const diskThreshold uint64 = 104857600 // Threshold is in bytes and it is equal to 100 MB
const inodeThreshold uint64 = 104858 // Threshold is in bytes and it is equal to approx 0.1 MB

var log = logging.MustGetLogger("ORK")

var deletableDirs = []string{"/opt/jumpscale7/var/log/", "/var/log/ovs/"}
var deletablePatterns = []string{"/var/log/syslog*"}

var customThreshold = map[string]Threshold{"/": Threshold{209715200, 524288}}

func addPartitionsDeletables(partitions []disk.PartitionStat, logs []string, deleteFunc func(string), partitionsDeletables map[disk.PartitionStat][]Deletable) {
	for _, path := range logs {
		var mpPartition disk.PartitionStat

		for _, partition := range partitions {
			if strings.HasPrefix(path, partition.Mountpoint) && len(partition.Mountpoint) > len(mpPartition.Mountpoint) {
				mpPartition = partition
			}
		}

		if mpPartition.Device == "" {
			log.Debugf("There is no mountpoint for %v", path)
			continue
		}
		partitionsDeletables[mpPartition] = append(partitionsDeletables[mpPartition], Deletable{path, deleteFunc})
	}
}

func makePartitionsMap() (map[disk.PartitionStat][]Deletable, error) {
	partitionsLogs := make(map[disk.PartitionStat][]Deletable)
	partitions, err := disk.Partitions(false)

	if err != nil {
		log.Error("Error getting disk partitions")
		return nil, err
	}

	addPartitionsDeletables(partitions, deletableDirs, utils.RemoveDirContents, partitionsLogs)
	addPartitionsDeletables(partitions, deletablePatterns, utils.RemoveFilesWithPattern, partitionsLogs)

	return partitionsLogs, nil
}

func Monitor() error {
	log.Info("Monitoring disk usage")

	var partDiskThreshold, partInodeThreshold uint64

	partitions, err := makePartitionsMap()
	if err != nil {
		return err
	}

	for partition, deletables := range partitions {
		if threshold, in := customThreshold[partition.Mountpoint]; in == true {
			partDiskThreshold = threshold.disk
			partInodeThreshold = threshold.inode
		} else {
			partDiskThreshold = diskThreshold
			partInodeThreshold = inodeThreshold
		}

		stats, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			log.Errorf("Error getting usage for mount point %v", partition.Mountpoint)
			continue
		}

		for i := 0; i < len(deletables) && (stats.Free < partDiskThreshold || stats.InodesFree < partInodeThreshold); i++ {
			log.Debugf("Partition %v at mounpoint %v exceeded usage threshold", partition.Device, partition.Mountpoint)

			deletable := deletables[i]
			deletable.deleteFunc(deletable.path)

			if stats, err = disk.Usage(partition.Mountpoint); err != nil {
				log.Errorf("Error getting usage for mount point %v", partition.Mountpoint)
				break
			}
		}
	}
	return nil
}
