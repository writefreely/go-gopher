package main

import (
	"flag"
	"log"
	"os"

	"github.com/prologic/go-gopher"
)

func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func main() {
	var (
		bind = flag.String("bind", ":70", "port to listen on")
		root = flag.String("root", cwd(), "root directory to serve")
	)

	flag.Parse()

	gopher.Handle("/", gopher.FileServer(gopher.Dir(*root)))

	log.Fatal(gopher.ListenAndServe(*bind, nil))
}
