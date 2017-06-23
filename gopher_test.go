package gopher_test

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/prologic/go-gopher"
	"github.com/stretchr/testify/assert"
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
	assert := assert.New(t)

	res, err := gopher.Get("gopher://localhost:7000/1hello")
	assert.Nil(err)

	b, err := res.Dir.ToText()
	assert.Nil(err)

	t.Logf("res: %s", string(b))

	assert.Len(res.Dir.Items, 1)

	assert.Equal(res.Dir.Items[0].Type, gopher.INFO)
	assert.Equal(res.Dir.Items[0].Description, "Hello World!")
}

func TestFileServer(t *testing.T) {
	assert := assert.New(t)

	res, err := gopher.Get("gopher://localhost:7000/")
	assert.Nil(err)
	assert.Len(res.Dir.Items, 5)

	json, err := res.Dir.ToJSON()
	assert.Nil(err)

	assert.JSONEq(string(json), `{"items":[{"type":"0","description":"LICENSE","selector":"LICENSE","host":"127.0.0.1","port":7000,"extras":null},{"type":"0","description":"README.md","selector":"README.md","host":"127.0.0.1","port":7000,"extras":null},{"type":"1","description":"examples","selector":"examples","host":"127.0.0.1","port":7000,"extras":null},{"type":"0","description":"gopher.go","selector":"gopher.go","host":"127.0.0.1","port":7000,"extras":null},{"type":"0","description":"gopher_test.go","selector":"gopher_test.go","host":"127.0.0.1","port":7000,"extras":null}]}`)
}

func TestMain(m *testing.M) {
	gopher.Handle("/", gopher.FileServer(gopher.Dir(".")))
	gopher.HandleFunc("/hello", hello)
	go func() {
		log.Fatal(gopher.ListenAndServe("localhost:7000", nil))
	}()

	os.Exit(m.Run())
}
