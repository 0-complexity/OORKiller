# OOR Killer - Out Of Resource Killer

## Role
The role of the OOR Killer is to be the last shield against an unresponsive machine.
In a ideal world, it should never have to take any action. This implies that the ORR Killer will be more aggresive then the selfhealing scripts.
It will not try to throttle down resource hungry processes but instead it will simply kill them. Its response should be as fast as possible in order to save the machine.
This also means that the OOR Killer is not a monitoring system. It just take action at an instant T cause the current status of the system is not acceptable anymore.

Still, the action taken should try to be the less harmfull for the system. To achive this we follow the same rules as the linux OOM killer, but adapted for our needs:
1. we lose the minimum amount of work done
2. we recover a large amount of resource
3. we want to kill the minimum amount of processes
4. we try to kill the process the user expects us to kill.

## Monitored resources

### Memory
If the amount of available memory for the system dropped under a certain limit. OOR Killer should determine the best process to kill to recover enough memory.

### CPU
If the load of the system reach a defined limit, OOR Killer should kill the process creating the load.

### Disk space
If the available disks space or free inode dropper under a certain limit, OOR killer should remove files to free up some space and eventually try to determine if a process is currently eating up disk space at an anormal rate.

List of files OOR Killer can delete:
- OVS logs
- To be defined...

### IOPS
To be defined

### Bandwidth
To be defined
