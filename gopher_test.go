package gopher_test

import (
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/writefreely/go-gopher"
)

var (
	testHost string = "localhost"
	testPort int
)

func pickUnusedPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		return 0, err
	}
	return port, nil
}

func TestMain(m *testing.M) {
	port, err := pickUnusedPort()
	if err != nil {
		log.Fatalf("error finding a free port: %s", err)
	}
	testPort = port

	go func() {
		gopher.Handle("/", gopher.FileServer(gopher.Dir("./testdata")))
		gopher.HandleFunc("/hello", hello)
		log.Printf("Test server starting on :%d\n", testPort)
		log.Fatal(gopher.ListenAndServe(fmt.Sprintf(":%d", testPort), nil))
	}()

	// Because it can take some time for the server to spin up
	// the tests are inconsistent - they'll fail if the server isn't
	// ready, but pass otherwise. This problem seems more pronounced
	// when running via the makefile.
	//
	// It seems like there should be a better way to do this
	for attempts := 3; attempts > 0; attempts-- {
		_, err := gopher.Get(fmt.Sprintf("gopher://%s:%d", testHost, testPort))
		if err == nil {
			log.Println("Server ready")
			break
		}
		log.Printf("Server not ready, going to try again in a sec. %v\n", err)
		time.Sleep(1 * time.Second)
	}

	code := m.Run()
	os.Exit(code)
}

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
	require := require.New(t)

	res, err := gopher.Get(fmt.Sprintf("gopher://%s:%d/1hello", testHost, testPort))
	require.NoError(err)
	assert.Len(res.Dir.Items, 1)
	assert.Equal(res.Dir.Items[0].Type, gopher.INFO)
	assert.Equal(res.Dir.Items[0].Description, "Hello World!")

	out, err := res.Dir.ToText()
	require.NoError(err)
	assert.Equal(string(out), "iHello World!\t\terror.host\t1\r\n")
}

func TestFileServer(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	res, err := gopher.Get(fmt.Sprintf("gopher://%s:%d/", testHost, testPort))
	require.NoError(err)
	assert.Len(res.Dir.Items, 1)

	json, err := res.Dir.ToJSON()
	require.NoError(err)

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
	require := require.New(t)

	item, err := gopher.ParseItem("0foo\t/foo\tlocalhost\t70\r\n")
	require.NoError(err)
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
	require := require.New(t)

	data := "0foo\t/foo\tlocalhost\t70\r\n"
	item, err := gopher.ParseItem(data)
	require.NoError(err)
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
	require := require.New(t)

	data := "0"
	item, err := gopher.ParseItem(data)
	require.NoError(err)
	assert.NotNil(item)

	data1, err := item.MarshalText()
	assert.Nil(err)

	item1, err := gopher.ParseItem(string(data1))
	assert.NoError(err)
	assert.NotNil(item1)
	assert.Equal(item, item1)
}
