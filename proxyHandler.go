package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"

	klog "k8s.io/klog/v2"
)

// ProxyHandler handle the http requests to be proxied.
// It implements http.Handler
type ProxyHandler struct{
	httpClient http.Client
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	process := findSenderProcess(r.RemoteAddr)
	switch r.Method {
	case "CONNECT":
		klog.Infof("%v (%v) %v %v", r.RemoteAddr, process, r.Method, r.URL.Host)
		p.serveVerbConnect(w, r)
	default:
		klog.Infof("%v (%v) %v %v://%v%v", r.RemoteAddr, process, r.Method, r.URL.Scheme, r.URL.Host, r.URL.Path)
		p.serveVerbOthers(w, r)
	}
}

func (p *ProxyHandler) serveVerbConnect(w http.ResponseWriter, r *http.Request) {

	remote, err := net.Dial("tcp", r.URL.Host)
	if err != nil {
		klog.Infof("error connecting to %v: %v", r.RemoteAddr, err)
		return
	}
	defer remote.Close()
	w.WriteHeader(200)

	hj, ok := w.(http.Hijacker)
	if !ok {
		klog.Infof("cannot cast ResponseWriter %v to http.Hijacker for CONNECT request to %v", w, r.RemoteAddr)
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		klog.Infof("cannot Hijack for CONNECT request to %v: %v", r.RemoteAddr, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	wg := &sync.WaitGroup{}
	copy := func(dst io.Writer, src io.Reader) {
		io.Copy(dst, src)
		//n, err := io.Copy(dst, src)
		//if err != nil {
		//	klog.Infof("written %v bytes and got error: %v" n, err)
		//do not log errors, too many:
		//"error copying data: readfrom tcp [::1]:8080->[::1]:56370: splice: broken pipe"
		// normal when for example the remote site close a connection
		//and
		//"error copying data: readfrom tcp 192.168.88.10:53592->216.58.207.78:443: read tcp [::1]:8080->[::1]:57138: read: connection reset by peer"
		// normal when for example the client side close the connection
		//}
		wg.Done()
	}
	wg.Add(2)
	go copy(bufrw, remote)
	go copy(remote, bufrw)
	wg.Wait()

}

func (p *ProxyHandler) serveVerbOthers(w http.ResponseWriter, r *http.Request) {

	newR := r.Clone(context.Background())
	newR.RequestURI = ""

	remoteResp, err := p.httpClient.Do(newR)
	if err != nil {
		klog.Infof("failed http request to %v: %v\n", newR.URL.Host, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := remoteResp.Body.Close(); err != nil {
			klog.Infof("failed to close response body for request to %v: %v", newR.URL.Host, err)
		}
	}()

	// copy header of remote response to the response of this request
	for k, vv := range remoteResp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(remoteResp.StatusCode)

	// copy body of remote response to the response of this request
	_, err = io.Copy(w, remoteResp.Body)
	if err != nil {
		klog.Infof("failed to copy remote response body to request for %v: %v", newR.URL.Host, err)
		return
	}

}
