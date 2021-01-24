package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"

	klog "k8s.io/klog/v2"
)

const (
	localhostIPv6Hex = "00000000000000000000000001000000" // [::1]
	localhostIPv4Hex = "0100007F"                         // 127.0.0.1
)

// findSenderProcess return information about the process that initiated a connection with a specific source port.
// remoteAddr is of the form <address:port>, only local addresses can return meaningful data, if the address is not local
// the function will return "remote". The function can still fail to return data, in that case "no_data" will be returned.
func findSenderProcess(remoteAddr string) string {
	split := strings.Split(remoteAddr, ":")
	if len(split) < 2 {
		klog.V(1).Infof("error parsing remote address, %v it is not of the form add:port", split)
		return "no_data"
	}
	remoteIP := strings.Join(split[:len(split)-1], ":")
	remotePort := split[len(split)-1]
	process := "remote"
	if remoteIP == "[::1]" {
		process = findProcessForSourcePort("/proc/net/tcp6", localhostIPv6Hex, remotePort)
	} else if remoteIP == "127.0.0.1" {
		process = findProcessForSourcePort("/proc/net/tcp", localhostIPv4Hex, remotePort)
	}
	return process
}

// findProcessForSourcePort return information about a process that initiated a connection with source port "remotePort" looking at 
// a file table of the form described here: https://www.kernel.org/doc/Documentation/networking/proc_net_tcp.txt, basically /proc/net/tcp6
// or /proc/net/tcp. "localhost" represent 
func findProcessForSourcePort(statFile string, localhost string, remotePort string) string {

	file, err := os.Open(statFile)
	if err != nil {
		klog.V(1).Infof("cannot open %v", statFile)
		return "no_data"
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		l := scanner.Text()
		fields := strings.Fields(l)
		if len(fields) < 10 {
			klog.V(1).Infof("error on reading %v: too few fields", statFile)
			return "no_data"
		}
		if fields[0] == "sl" {
			continue
		}
		localEndpoint := fields[1]
		split := strings.Split(localEndpoint, ":")
		if len(split) != 2 {
			klog.V(1).Infof("error on reading %v: cannot define local address: %v", statFile, localEndpoint)
			return "no_data"
		}
		localAddress := split[0]
		localPortHex := split[1]
		localPortHexNum, err := strconv.ParseInt(localPortHex, 16, 64)
		if err != nil {
			klog.V(1).Infof("cannot convert %v to int64: %v", localPortHex, err)
			return "no_data"
		}
		localPortDec := fmt.Sprint(localPortHexNum)
		inode := fields[9]
		if localAddress == localhost && localPortDec == remotePort {
			if process, found := findProcessFromInode(inode); found {
				return process
			}
			klog.V(1).Infof("got inode %v for remote port %v but no process found with that inode", inode, remotePort)
			return "no_data"
		}
	}

	if err := scanner.Err(); err != nil {
		klog.V(1).Infof("error reading %v", statFile)
		return "no_data"
	}

	klog.V(1).Infof("no fd found for port %v in %v", remotePort, statFile)
	return "no_data"

}

// findProcessFromInode returns the process description (of the form process_name@pid) of a process that is holding a file that points to 
// the inode defined as string as parameter of this function, and true, or ("no_data", false) if no such process is found.
//
// To do so it scans for processs in the /proc dir, for each process it scans the fd dir, for each fd it checks if the inode it points to is the 
// one we are looking for.
//
// To have an accurate search the process that do this opertation must be root, otherwise only processes of the current uid can be scanned.
func findProcessFromInode(inode string) (string, bool) {

	inodeUint64, err := strconv.ParseUint(inode, 10, 64)
	if err != nil {
		klog.Infof("cannot convert inode %v to uint64", inode)
		return "no_data", false
	}

	d, err := os.Open("/proc")
	if err != nil {
		klog.V(1).Infof("cannot open dir /proc: %v", err)
		return "no_data", false
	}
	defer d.Close()
	files, err := d.Readdir(-1)
	if err != nil {
		klog.V(1).Infof("cannot list dir /proc: %v", err)
		return "no_data", false
	}

	for _, f := range files {

		if _, err := strconv.ParseInt(f.Name(), 10, 64); !f.IsDir() || err != nil {
			continue
		}

		processDirName := "/proc/" + f.Name()
		processDirFdName := processDirName + "/fd"
		processDir, err := os.Open(processDirFdName)
		if err != nil {
			klog.V(1).Infof("cannot open dir %v: %v", processDirFdName, err)
			continue
		}
		defer processDir.Close()

		fdFiles, err := processDir.Readdirnames(-1)
		if err != nil {
			klog.V(1).Infof("cannot list dir %v: %v", processDirFdName, err)
			continue
		}
		for _, fdFile := range fdFiles {
			absPath := processDirFdName + "/" + fdFile
			statt := new(syscall.Stat_t)
			err = syscall.Stat(absPath, statt)
			if err != nil {
				continue
			}
			if statt.Ino == inodeUint64 {
				processCommName := processDirName + "/comm"
				comm, err := os.Open(processCommName)
				if err != nil {
					klog.V(1).Infof("cannot open file %v: %v", processCommName, err)
					continue
				}
				defer comm.Close()
				data, err := ioutil.ReadAll(comm)
				return string(data[:len(data)-1]) + "@" + f.Name(), true
			}
		}

	}

	return "no_data", false
}