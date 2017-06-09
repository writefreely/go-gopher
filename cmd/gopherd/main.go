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
		host = flag.String("host", "localhost", "fqdn hostname")
		root = flag.String("root", cwd(), "root directory to serve")
	)

	flag.Parse()

	gopher.Handle("/", gopher.FileServer(gopher.Dir(*root)))

	server := gopher.Server{Addr: *bind, Hostname: *host}
	log.Fatal(server.ListenAndServe())
}
