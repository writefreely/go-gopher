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
	assert.NoError(err)
	assert.Len(res.Dir.Items, 1)
	assert.Equal(res.Dir.Items[0].Type, gopher.INFO)
	assert.Equal(res.Dir.Items[0].Description, "Hello World!")

	out, err := res.Dir.ToText()
	assert.NoError(err)
	assert.Equal(string(out), "iHello World!\t\terror.host\t1\r\n")
}

func TestFileServer(t *testing.T) {
	assert := assert.New(t)

	res, err := gopher.Get("gopher://localhost:7000/")
	assert.NoError(err)
	assert.Len(res.Dir.Items, 5)

	json, err := res.Dir.ToJSON()
	assert.Nil(err)

	assert.JSONEq(string(json), `{"items":[{"type":"0","description":"LICENSE","selector":"LICENSE","host":"127.0.0.1","port":7000,"extras":[]},{"type":"0","description":"README.md","selector":"README.md","host":"127.0.0.1","port":7000,"extras":[]},{"type":"1","description":"examples","selector":"examples","host":"127.0.0.1","port":7000,"extras":[]},{"type":"0","description":"gopher.go","selector":"gopher.go","host":"127.0.0.1","port":7000,"extras":[]},{"type":"0","description":"gopher_test.go","selector":"gopher_test.go","host":"127.0.0.1","port":7000,"extras":[]}]}`)
}

func TestParseItemNull(t *testing.T) {
	assert := assert.New(t)

	item, err := gopher.ParseItem("")
	assert.Nil(item)
	assert.Error(err)
}

func TestParseItem(t *testing.T) {
	assert := assert.New(t)

	item, err := gopher.ParseItem("0foo\t/foo\tlocalhost\t70\r\n")
	assert.NoError(err)
	assert.NotNil(item)
	assert.Equal(item, &gopher.Item{
		Type:        gopher.FILE,
		Description: "foo",
		Selector:    "/foo",
		Host:        "localhost",
		Port:        70,
		Extras:      []string{},
	})
}

func TestParseItemMarshal(t *testing.T) {
	assert := assert.New(t)

	data := "0foo\t/foo\tlocalhost\t70\r\n"
	item, err := gopher.ParseItem(data)
	assert.NoError(err)
	assert.NotNil(item)
	assert.Equal(item, &gopher.Item{
		Type:        gopher.FILE,
		Description: "foo",
		Selector:    "/foo",
		Host:        "localhost",
		Port:        70,
		Extras:      []string{},
	})

	data1, err := item.MarshalText()
	assert.Nil(err)
	assert.Equal(data, string(data1))
}

func TestParseItemMarshalIdempotency(t *testing.T) {
	assert := assert.New(t)

	data := "0"
	item, err := gopher.ParseItem(data)
	assert.NoError(err)
	assert.NotNil(item)

	data1, err := item.MarshalText()
	assert.Nil(err)

	item1, err := gopher.ParseItem(string(data1))
	assert.NoError(err)
	assert.NotNil(item1)
	assert.Equal(item, item1)
}

func TestMain(m *testing.M) {
	gopher.Handle("/", gopher.FileServer(gopher.Dir(".")))
	gopher.HandleFunc("/hello", hello)
	go func() {
		log.Fatal(gopher.ListenAndServe("localhost:7000", nil))
	}()

	os.Exit(m.Run())
}
