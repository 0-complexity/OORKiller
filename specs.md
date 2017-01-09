# ORK - Out of Resource Killer

## Role
The role of the ORK is to be the last shield against an unresponsive machine.
In a ideal world, it should never have to take any action. This implies that the ORK will be more aggresive then the selfhealing scripts.
It will not try to throttle down resource hungry processes but instead it will simply kill them. Its response should be as fast as possible in order to save the machine.
This also means that the ORK is not a monitoring system. It just take action at an instant T cause the current status of the system is not acceptable anymore.

Still, the action taken should try to be the less harmfull for the system. To achive this we follow the same rules as the linux OOM killer, but adapted for our needs:

1. we lose the minimum amount of work done
2. we recover a large amount of resource
3. we want to kill the minimum amount of processes
4. we try to kill the process the user expects us to kill.

## Monitored resources

### Memory
If the amount of available memory for the system dropped under a certain limit. ORK should determine the best process to kill to recover enough memory.


ORK will monitor the current memory by reading `/proc/meminfo` every second.
If the memory threshold is reached
- 1: ORK will look into the running VM and hibernate to disk the VMs that consume the more memory. 
- 2: If `1` is not enough, ORK will have to loop over all processes in `/proc` and search for the process to kill

### CPU
If the load of the system reach a defined limit, ORK should kill the process creating the load.

OORkill will monitor the current CPU usage by reading `/proc/stat` every second.
If the CPU theshold is reached:
- 1: ORK will look into the running VM and freeze the VMs consuming more CPU.
- 2: if `1` is not enough, ORK will have to loop over all processes in `/proc` and search for the process to kill.

### Disk space
If the available disks space or free inode dropper under a certain limit, ORK should remove files to free up some space and eventually try to determine if a process is currently eating up disk space at an anormal rate.

Each disks can have a different configuration. Root disk should not have the same threshold as a SSD used as cache.

List of files ORK can delete:
- OVS logs
- /var/log/syslog
- /opt/jumpscale7/var/log

ORK will monitor all the disks using the `stat /dev/sdx` command every 5 seconds.
If the threshold of the disks is reached the ORK will try to free up some space by:
- 1. looking up in predefined location where we know we can delete files.
- 2. ...


### IOPS
To be defined

### Network
Check for duplicated mac addresses between all nodes.  
ORK should monitor the ARP packets and detect duplicated mac addresses. ORK put the duplicated interface down.

## Architecture
- All the thresholds and limits should hardcoded in the code, this is too simplfy deployement and have less mangement around the ORK.
- the ORK is running as a daemon started at boot.
- The execution of an action for a given ressources should not block the monitoring of the other ressources
- logging: The ORK can use the kernel logs (/dev/kmsg) to write what action has been taken. If any other process wants to monitor the actions taken by the ORK it has to read the kernel logs. We choose Kernel logs because this is a pre-allocated ring buffer, thus we can always write to it.
