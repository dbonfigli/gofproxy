
package main

import (
	"syscall"
	klog "k8s.io/klog/v2"
)

// setLimits increase the limit of opened files to the maximum possible
func setLimits() {
	res := new(syscall.Rlimit)
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, res); err != nil {
		klog.Infof("cannot get limits for RLIMIT_NOFILE: %v", err)
		return
	}
	res.Cur = res.Max
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, res); err != nil {
		klog.Infof("cannot set limits for RLIMIT_NOFILE: %v", err)
	}
}