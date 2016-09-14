package gopher_test

import (
	"fmt"
	"log"

	"github.com/prologic/go-gopher"
)

func hello(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteInfo("Hello World!")
}

func ExampleGet() {
	res, err := gopher.Get("gopher://gopher.floodgap.com")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(res.Dir.ToText())
}

func ExampleServer() {
	gopher.HandleFunc("/hello", hello)
	log.Fatal(gopher.ListenAndServe("localhost:7000", nil))
}

func ExampleFileServer() {
	gopher.Handle("/", gopher.FileServer(gopher.Dir("/tmp")))

	log.Fatal(gopher.ListenAndServe("localhost:7000", nil))
}
