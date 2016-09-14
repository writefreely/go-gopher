package main

import (
	"log"

	"github.com/prologic/go-gopher"
)

func index(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteItem(
		gopher.Item{
			Type:        gopher.FILE,
			Selector:    "/hello",
			Description: "hello",
		},
	)
}

func hello(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteInfo("Hello World!")
}

func main() {
	gopher.HandleFunc("/", index)
	gopher.HandleFunc("/hello", hello)
	log.Fatal(gopher.ListenAndServe("localhost:70", nil))
}
