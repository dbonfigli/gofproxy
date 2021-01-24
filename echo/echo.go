package main

import (
	"fmt"
	"net/http"
	"io/ioutil"
)

func headers(w http.ResponseWriter, req *http.Request) {

	fmt.Printf("got headers:\n")
	w.WriteHeader(500)
	fmt.Fprintf(w, "got headers:\n")
	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Printf("%v: %v\n", name, h)
			fmt.Fprintf(w, "%v: %v\n", name, h)
		}
	}
	
	body, _ := ioutil.ReadAll(req.Body)
	fmt.Printf("got body: %s\n", body)
	fmt.Fprintf(w, "body: %s\n", body)

}

func main() {
	http.HandleFunc("/", headers)
	http.ListenAndServe(":8090", nil)
}