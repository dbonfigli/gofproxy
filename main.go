package main

import (
	"flag"
	"net/http"
	"time"

	klog "k8s.io/klog/v2"
)

func main() {

	// initialize klog
	klog.InitFlags(nil)
	flag.Parse()

	// upgrade limits to the maximum possible, the proxy use a lot of files...
	setLimits()

	// disable proxy configuration in env variables
	noProxyDefaultTransport := http.DefaultTransport.(*http.Transport)
	noProxyDefaultTransport.Proxy = nil
	handler := &ProxyHandler{
		httpClient: http.Client{
			Timeout: 5 * time.Hour,
			Transport: noProxyDefaultTransport,
		},
	}

	// start the proxy
	klog.Infof("starting proxy...")
	klog.Fatal(http.ListenAndServe(":8080", handler))
}
