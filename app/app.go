package main

import (
	"github.com/glaslos/names"
)

func main() {
	n, err := names.New()
	if err != nil {
		panic(err)
	}
	n.Run()
	n.Log.Printf("exiting.\n")
}
