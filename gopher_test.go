package gopher_test

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/prologic/go-gopher"
)

func hello(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteInfo("Hello World!")
}

func Example_client() {
	res, err := gopher.Get("gopher://gopher.floodgap.com")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(res.Dir.ToText())
}

func Example_server() {
	gopher.HandleFunc("/hello", hello)
	log.Fatal(gopher.ListenAndServe("localhost:7000", nil))
}

func Example_fileserver() {
	gopher.Handle("/", gopher.FileServer(gopher.Dir("/tmp")))
	log.Fatal(gopher.ListenAndServe("localhost:7000", nil))
}

func TestGet(t *testing.T) {
	res, err := gopher.Get("gopher://localhost:7000/1hello")
	if err != nil {
		t.Fatal(err)
	}

	b, err := res.Dir.ToText()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("res: %s", string(b))

	if len(res.Dir) == 0 {
		t.Fatal("expected items but none found")
	}

	i := res.Dir[0]
	if i.Type != gopher.INFO {
		log.Fatalf("expected INFO item %s found", i.Type)
	}

	if i.Description != "Hello World!" {
		log.Fatal("expected \"Hello World!\" as description")
	}
}

func TestMain(m *testing.M) {
	log.Print("Setup...")

	gopher.HandleFunc("/hello", hello)
	go func() {
		log.Fatal(gopher.ListenAndServe("localhost:7000", nil))
	}()

	retcode := m.Run()
	log.Printf(" Return: %q", retcode)

	log.Print("Teardown...")

	os.Exit(retcode)
}
