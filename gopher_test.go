package gopher_test

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"git.mills.io/prologic/go-gopher"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	ch := startTestServer()
	defer stopTestServer(ch)

	// Because it can take some time for the server to spin up
	// the tests are inconsistent - they'll fail if the server isn't
	// ready, but pass otherwise. This problem seems more pronounced
	// when running via the makefile.
	//
	// It seems like there should be a better way to do this
	for attempts := 3; attempts > 0; attempts-- {
		_, err := gopher.Get("gopher://localhost:7000")
		if err == nil {
			fmt.Println("Server ready")
			break
		}
		fmt.Printf("Server not ready, going to try again in a sec. %v", err)
		time.Sleep(1 * time.Millisecond)
	}
	/////

	code := m.Run()
	os.Exit(code)
}

func hello(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteInfo("Hello World!")
}

func startTestServer() chan bool {
	quit := make(chan bool)
	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				gopher.Handle("/", gopher.FileServer(gopher.Dir("./testdata")))
				gopher.HandleFunc("/hello", hello)
				log.Println("Test server starting on 7000")
				err := gopher.ListenAndServe("localhost:7000", nil)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}()

	return quit
}

func stopTestServer(c chan bool) {
	c <- true
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
	assert.Len(res.Dir.Items, 1)

	json, err := res.Dir.ToJSON()
	assert.Nil(err)

	log.Println(string(json))
	assert.JSONEq(
		`{"items":[{"type":"0","description":"hello.txt","selector":"/hello.txt","host":"127.0.0.1","port":7000,"extras":[]}]}`,
		string(json))
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
