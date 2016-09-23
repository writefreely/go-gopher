package main

import (
	"log"

	"github.com/prologic/go-gopher"
)

func index(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteItem(
		gopher.Item{
			Type:        gopher.DIRECTORY,
			Selector:    "/hello",
			Description: "hello",
		},
	)
	w.WriteItem(
		gopher.Item{
			Type:        gopher.FILE,
			Selector:    "/foo",
			Description: "foo",
		},
	)
	w.WriteItem(
		gopher.Item{
			Type:        gopher.DIRECTORY,
			Selector:    "/",
			Description: "Floodgap",
			Host:        "gopher.floodgap.com",
			Port:        70,
		},
	)
}

func hello(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteInfo("Hello World!")
}

func foo(w gopher.ResponseWriter, r *gopher.Request) {
	w.Write([]byte("Foo!"))
}

func main() {
	gopher.HandleFunc("/", index)
	gopher.HandleFunc("/foo", foo)
	gopher.HandleFunc("/hello", hello)
	log.Fatal(gopher.ListenAndServe("localhost:70", nil))
}
