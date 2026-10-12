// Command catalogcheck reads private catalog trees the way the server does at
// boot, against the catalog the binary ships, and reports every problem
// without touching a database. The trees are checked together, as one owner's.
//
//	go run ./server/cmd/catalogcheck <tree>...
package main

import (
	"fmt"
	"os"

	shipped "passion/catalog"
	"passion/server/catalog"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: catalogcheck <tree>...")
		os.Exit(2)
	}

	warnings, err := check(os.Args[1:]...)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(locations ...string) ([]string, error) {
	trees := make([]catalog.Tree, 0, len(locations))
	for _, l := range locations {
		trees = append(trees, catalog.Tree{Name: l, FS: os.DirFS(l)})
	}
	app := catalog.Tree{Name: "catalog", FS: shipped.Files}

	exercises, err := catalog.Read(trees...)
	if err != nil {
		return nil, err
	}
	_, warnings, err := catalog.ReadSessions(&app, exercises, trees...)
	return warnings, err
}
