package serve

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/hcchien/ailog/internal/build"
	"github.com/hcchien/ailog/internal/config"
)

// Run starts a local file server over dist/, watching content/themes/public for
// changes and rebuilding on the fly. Intended for preview, not production.
func Run(site *config.Site, addr string) error {
	if err := build.Run(site); err != nil {
		return fmt.Errorf("initial build: %w", err)
	}

	var mu sync.Mutex
	rebuild := func() {
		mu.Lock()
		defer mu.Unlock()
		start := time.Now()
		if err := build.Run(site); err != nil {
			log.Printf("rebuild error: %v", err)
			return
		}
		log.Printf("rebuilt in %s", time.Since(start))
	}

	go watch(site, rebuild)

	fs := http.FileServer(http.Dir(site.DistDir()))
	mux := http.NewServeMux()
	handler := noCache(notFoundHandler(site, fs))
	prefix := site.BasePath
	if prefix == "" {
		mux.Handle("/", handler)
	} else {
		mux.Handle(prefix+"/", http.StripPrefix(prefix, handler))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, prefix+"/", http.StatusFound)
		})
	}

	log.Printf("ailog serve → http://%s%s/ (root %s)", addr, prefix, site.DistDir())
	return http.ListenAndServe(addr, mux)
}

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// notFoundHandler falls back to dist/404.html when a path isn't found.
func notFoundHandler(site *config.Site, fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(site.DistDir(), filepath.Clean(r.URL.Path))
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(path, "index.html")); err == nil {
				fs.ServeHTTP(w, r)
				return
			}
		} else if err == nil {
			fs.ServeHTTP(w, r)
			return
		}
		if !strings.HasSuffix(r.URL.Path, "/") && !strings.Contains(filepath.Base(r.URL.Path), ".") {
			fs.ServeHTTP(w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		f, err := os.Open(filepath.Join(site.DistDir(), "404.html"))
		if err != nil {
			fmt.Fprintln(w, "404 not found")
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(readAll(f))
	})
}

func readAll(f *os.File) []byte {
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	buf := make([]byte, info.Size())
	_, _ = f.Read(buf)
	return buf
}

func watch(site *config.Site, onChange func()) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watcher disabled: %v", err)
		return
	}
	defer w.Close()

	dirs := []string{
		site.PostsDir(),
		filepath.Join(site.ThemeDir(), "layouts"),
		filepath.Join(site.ThemeDir(), "static"),
		site.PublicDir(),
	}
	for _, d := range dirs {
		_ = filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			if info.IsDir() {
				_ = w.Add(path)
			}
			return nil
		})
	}

	// debounce so a single save doesn't trigger N rebuilds
	var debounceTimer *time.Timer
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Chmod == event.Op {
				continue
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(200*time.Millisecond, onChange)
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Printf("watch error: %v", err)
		}
	}
}
