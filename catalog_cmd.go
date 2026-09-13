package main

// The catalog subcommand: checking a tree of files without a database, a server or a
// config, so a private catalog and CI can both run the same checks.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"passion/store"
)

const catalogUsage = `usage: passion catalog lint [--fix] <dir>...

Checks a catalog tree and prints what is wrong with it. Exits 1 when anything is.

  --fix   write an id into every file that has none, then check

Give more than one directory and the first is treated as the app's catalog, so the others
may reference it with app:<slug>. With one directory an app: reference is accepted without
being checked, because there is nothing to check it against.
`

// runCatalog handles `passion catalog ...`. It returns the process exit code.
func runCatalog(args []string) int {
	if len(args) == 0 || args[0] != "lint" {
		fmt.Fprint(os.Stderr, catalogUsage)
		return 2
	}

	fs := flag.NewFlagSet("catalog lint", flag.ExitOnError)
	fix := fs.Bool("fix", false, "write an id into every file that has none")
	fs.Usage = func() { fmt.Fprint(os.Stderr, catalogUsage) }
	_ = fs.Parse(args[1:])

	dirs := fs.Args()
	if len(dirs) == 0 {
		fmt.Fprint(os.Stderr, catalogUsage)
		return 2
	}

	var problems []string
	var trees []*store.Tree

	for i, dir := range dirs {
		if *fix {
			wrote, err := store.MintIDs(dir)
			if err != nil {
				problems = append(problems, err.Error())
				continue
			}
			for _, name := range wrote {
				fmt.Printf("wrote an id into %s\n", name)
			}
		}

		// The first tree is the app's, so the rest can name it. A tree that fails to load is
		// reported and the run continues, so one bad file does not hide the next.
		var known map[store.Ref]bool
		if i > 0 && len(trees) > 0 && trees[0] != nil {
			known = trees[0].Index(true)
		}
		t, err := store.Load(os.DirFS(dir), filepath.Base(dir), known)
		if err != nil {
			problems = append(problems, prefix(dirs, dir, err.Error()))
			trees = append(trees, nil)
			continue
		}
		trees = append(trees, t)
	}

	loaded := trees[:0:0]
	for _, t := range trees {
		if t != nil {
			loaded = append(loaded, t)
		}
	}
	problems = append(problems, store.UnknownFamilies(loaded...)...)

	if len(problems) == 0 {
		fmt.Printf("%s: no problems\n", strings.Join(dirs, ", "))
		return 0
	}
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}
	fmt.Fprintf(os.Stderr, "%s\n", plural(len(problems), "problem"))
	return 1
}

// prefix names the directory a problem came from, and only when more than one is being
// checked. Load already names the file inside the tree.
func prefix(dirs []string, dir, msg string) string {
	if len(dirs) < 2 || strings.HasPrefix(msg, dir) {
		return msg
	}
	return dir + ": " + msg
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
