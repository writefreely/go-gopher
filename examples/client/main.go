package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"git.mills.io/prologic/go-gopher"
)

var (
	json = flag.Bool("json", false, "display gopher directory as JSON")
)

func main() {
	var uri string

	flag.Parse()

	if len(flag.Args()) == 1 {
		uri = flag.Arg(0)
	} else {
		uri = "gopher://gopher.floodgap.com/"
	}

	res, err := gopher.Get(uri)
	if err != nil {
		log.Fatal(err)
	}

	if res.Body != nil {
		contents, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Print(string(contents))
	} else {
		var (
			bytes []byte
			err   error
		)

		if *json {
			bytes, err = res.Dir.ToJSON()
		} else {
			bytes, err = res.Dir.ToText()
		}
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(bytes))
	}
}
