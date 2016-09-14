# Gopher protocol library for Golang

[![Build Status](https://travis-ci.org/prologic/go-gopher.svg)](https://travis-ci.org/prologic/go-gopher)

This is a standards compliant Gopher library for the Go programming language
implementing the RFC 1436 specification. The library includes both client and
server handling and examples of each.

## Installation
  
  $ go get github.com/prologic/go-gopher

## Usage

```#!go
import "github.com/prologic/go-gopher"
```

## Example

### Client

```#!go
package main

import (
	"fmt"

	"github.com/prologic/go-gopher"
)

func main() {
	res, _ := gopher.Get("gopher://gopher.floodgap.com/")
	bytes, _ = res.Dir.ToText()
	fmt.Println(string(bytes))
}
```

### Server

```#!go
package main

import (
	"log"

	"github.com/prologic/go-gopher"
)

func hello(w gopher.ResponseWriter, r *gopher.Request) {
	w.WriteInfo("Hello World!")
}

func main() {
	gopher.HandleFunc("/hello", hello)
	log.Fatal(gopher.ListenAndServe("localhost:70", nil))
}
```

## License

MIT
