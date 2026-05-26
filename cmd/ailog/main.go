package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hcchien/ailog/internal/admin"
	"github.com/hcchien/ailog/internal/build"
	"github.com/hcchien/ailog/internal/config"
	"github.com/hcchien/ailog/internal/content"
	"github.com/hcchien/ailog/internal/serve"
)

const usage = `ailog — a tiny Go-powered blog system

Usage:
  ailog build              build the static site into dist/
  ailog serve [addr]       preview locally with live reload (default :4321)
  ailog admin              start the local admin server (browser editor)
  ailog new "<title>"      create a new draft post under content/posts/

Flags:
  -c, --config <path>      path to ailog.yaml (default: ./ailog.yaml)
`

func main() {
	log.SetFlags(log.Ltime)

	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	var cfgPath string
	fs := flag.NewFlagSet("ailog", flag.ExitOnError)
	fs.StringVar(&cfgPath, "config", "ailog.yaml", "path to ailog.yaml")
	fs.StringVar(&cfgPath, "c", "ailog.yaml", "path to ailog.yaml (shorthand)")

	sub := os.Args[1]
	args := os.Args[2:]

	// pull out global flags from the subcommand args
	rest, err := splitFlags(fs, args)
	if err != nil {
		log.Fatal(err)
	}

	switch sub {
	case "build":
		site := must(config.Load(cfgPath))
		start := time.Now()
		if err := build.Run(site); err != nil {
			log.Fatal(err)
		}
		log.Printf("built %s in %s", site.DistDir(), time.Since(start))

	case "serve":
		site := must(config.Load(cfgPath))
		addr := ":4321"
		if len(rest) > 0 {
			addr = rest[0]
		}
		if err := serve.Run(site, addr); err != nil {
			log.Fatal(err)
		}

	case "admin":
		site := must(config.Load(cfgPath))
		if err := admin.Run(site); err != nil {
			log.Fatal(err)
		}

	case "new":
		if len(rest) < 1 {
			log.Fatal("usage: ailog new \"<title>\"")
		}
		title := strings.Join(rest, " ")
		site := must(config.Load(cfgPath))
		if err := newPost(site, title); err != nil {
			log.Fatal(err)
		}

	case "-h", "--help", "help":
		fmt.Print(usage)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", sub)
		fmt.Print(usage)
		os.Exit(2)
	}
}

func splitFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var flagArgs, rest []string
	skipNext := false
	for i, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		switch {
		case a == "-c" || a == "--config":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing value for %s", a)
			}
			flagArgs = append(flagArgs, a, args[i+1])
			skipNext = true
		case strings.HasPrefix(a, "--config="):
			flagArgs = append(flagArgs, a)
		default:
			rest = append(rest, a)
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		return nil, err
	}
	return rest, nil
}

func newPost(site *config.Site, title string) error {
	slug := content.Slugify(title)
	path := filepath.Join(site.PostsDir(), slug+".md")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("post already exists: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fm := content.Frontmatter{
		Title: title,
		Date:  time.Now(),
		Draft: true,
	}
	body := "\n寫點什麼…\n"
	out, err := content.Serialize(fm, body)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	fmt.Println("created", path)
	return nil
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return v
}
