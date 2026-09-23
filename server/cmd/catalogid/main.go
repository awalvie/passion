// Command catalogid gives every file in a catalog tree that has no id: line one
// of its own. Run it on a new file before it is committed.
//
//	go run ./server/cmd/catalogid <tree>
package main

import (
	"fmt"
	"os"

	"passion/server/catalog"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: catalogid <tree>")
		os.Exit(2)
	}

	changed, err := catalog.AddIDs(os.Args[1])
	for _, path := range changed {
		fmt.Println("added an id to", path)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
