package main

import (
	"log"
	"sync"

	"github.com/prologic/go-gopher"
)

func index(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteItem(&gopher.Item{
		Type:        gopher.DIRECTORY,
		Selector:    "/hello",
		Description: "hello",

		// TLS Resource
		Host:   "localhost",
		Port:   73,
		Extras: []string{"TLS"},
	})
	w.WriteItem(&gopher.Item{
		Type:        gopher.FILE,
		Selector:    "/foo",
		Description: "foo",
	})
	w.WriteItem(&gopher.Item{
		Type:        gopher.DIRECTORY,
		Selector:    "/",
		Description: "Floodgap",
		Host:        "gopher.floodgap.com",
		Port:        70,
	})
}

func hello(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteInfo("Hello World!")
}

func foo(w gopher.ResponseWriter, r *gopher.Request) {
	w.Write([]byte("Foo!"))
}

func main() {
	wg := &sync.WaitGroup{}

	// Standard Server
	wg.Add(1)
	go func() {
		mux := gopher.NewServeMux()

		mux.HandleFunc("/", index)
		mux.HandleFunc("/foo", foo)
		mux.HandleFunc("/hello", hello)

		log.Fatal(gopher.ListenAndServe("localhost:70", mux))
		wg.Done()
	}()

	// TLS server
	wg.Add(1)
	go func() {
		mux := gopher.NewServeMux()

		mux.HandleFunc("/", index)
		mux.HandleFunc("/foo", foo)
		mux.HandleFunc("/hello", hello)

		log.Fatal(
			gopher.ListenAndServeTLS(
				"localhost:73", "cert.pem", "key.pem", mux,
			),
		)
		wg.Done()
	}()

	wg.Wait()
}
