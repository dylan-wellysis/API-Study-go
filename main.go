package main

import (
	"fmt"
	"net/http"
)

func main() {
	// write any response with http.ResponseWriter
	// handle any request with http.Request
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Hello World"))
		fmt.Println(req.URL)
	})

	// In Go, nil is the zero value for pointers, interfaces, maps, slices, channels and function types, representing an uninitialized value.
	// second arg: ServeMux. if nil, DefaultServeMux is set
	http.ListenAndServe(":8080", nil)
}
