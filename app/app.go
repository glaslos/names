package main

import (
	"github.com/glaslos/names"
)

func main() {
	n := names.New()
	n.Run()
	n.Log.Printf("Exiting.\n")
}
