package main

// The catalog subcommand: checking a tree of files without a database, a server or a
// config, so a private catalog and CI can both run the same checks.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"passion/config"
	"passion/store"
)

const catalogUsage = `usage: passion catalog lint [--fix] <dir>...
       passion catalog export [--config <file>] [--owner <email>] <tree> <dir>

lint checks a catalog tree and prints what is wrong with it. It reads files and nothing
else: no database, no server, no config. Exits 1 when anything is wrong.

  --fix   write an id into every file that has none, then check

Give more than one directory and the first is treated as the app's catalog, so the others
may reference it with app:<slug>. With one directory an app: reference is accepted without
being checked, because there is nothing to check it against.

export writes a tree back out of the database as files, so you can diff what the app holds
against what your files say. <tree> is the tree name an import logged. Without --owner it
exports the rows the app ships.

  --config  path to a passion.yaml
  --owner   export this account's rows instead of the app's
`

// runCatalog handles `passion catalog ...`. It returns the process exit code.
func runCatalog(args []string) int {
	if len(args) > 0 && args[0] == "export" {
		return runCatalogExport(args[1:])
	}
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

func runCatalogExport(args []string) int {
	fs := flag.NewFlagSet("catalog export", flag.ExitOnError)
	configPath := fs.String("config", "", "path to a passion.yaml")
	owner := fs.String("owner", "", "export this account's rows instead of the app's")
	fs.Usage = func() { fmt.Fprint(os.Stderr, catalogUsage) }
	_ = fs.Parse(args)

	if fs.NArg() != 2 {
		fmt.Fprint(os.Stderr, catalogUsage)
		return 2
	}
	tree, dir := fs.Arg(0), fs.Arg(1)

	cfg, err := config.LoadApp(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "passion:", err)
		return 1
	}

	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{
		Engine:       cfg.Database.Engine,
		DSN:          cfg.Database.DSN,
		MaxOpenConns: cfg.Database.MaxOpenConns,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "passion:", err)
		return 1
	}
	defer func() { _ = st.Close() }()

	var files map[string][]byte
	if *owner == "" {
		files, err = st.ExportShipped(ctx, tree)
	} else {
		files, err = st.ExportOwned(ctx, tree, *owner)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "passion:", err)
		listTrees(ctx, st)
		return 1
	}

	stale, err := writeTree(dir, files)
	if err != nil {
		fmt.Fprintln(os.Stderr, "passion:", err)
		return 1
	}

	fmt.Printf("wrote %s into %s\n", plural(len(files), "file"), dir)
	for _, name := range stale {
		fmt.Printf("already there and not written by this export: %s\n", name)
	}
	return 0
}

// A file already in dir that this export did not write is reported, never deleted: only a
// person can say whether it is a leftover or something the database has never seen.
func writeTree(dir string, files map[string][]byte) ([]string, error) {
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return nil, err
		}
	}

	var stale []string
	for _, sub := range []string{".", "movements", "blocks", "sessions"} {
		entries, err := os.ReadDir(filepath.Join(dir, sub))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
				continue
			}
			name := e.Name()
			if sub != "." {
				name = sub + "/" + name
			}
			if _, wrote := files[name]; !wrote {
				stale = append(stale, name)
			}
		}
	}
	sort.Strings(stale)
	return stale, nil
}

func listTrees(ctx context.Context, st *store.Store) {
	trees, err := st.Trees(ctx)
	if err != nil || len(trees) == 0 {
		return
	}
	fmt.Fprintln(os.Stderr, "\nTrees in this database:")
	for _, t := range trees {
		who := "the app ships it"
		if t.Owner != "" {
			who = "owned by " + t.Owner
		}
		fmt.Fprintf(os.Stderr, "  %-28s %4d rows, %s\n", t.Name, t.RowCount, who)
	}
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
