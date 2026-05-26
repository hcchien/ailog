package admin

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/hcchien/ailog/internal/auth"
	"github.com/hcchien/ailog/internal/build"
	"github.com/hcchien/ailog/internal/config"
)

type Server struct {
	site *config.Site
	auth *auth.Manager
	md   markdownRenderer
}

type markdownRenderer interface {
	Render(body string) (template.HTML, error)
}

type goldmarkRenderer struct{}

func (goldmarkRenderer) Render(body string) (template.HTML, error) {
	return build.RenderMarkdown(build.Markdown(), body)
}

// Run starts the admin HTTP server on 127.0.0.1:<configured port>.
func Run(site *config.Site) error {
	if site.Admin.User == "" {
		return fmt.Errorf("ailog.yaml admin.user is empty — set it to the GitHub login allowed to use admin")
	}
	s := &Server{
		site: site,
		auth: auth.NewManager(site.Admin.User),
		md:   goldmarkRenderer{},
	}

	mux := http.NewServeMux()
	s.routes(mux)

	addr := fmt.Sprintf("127.0.0.1:%d", site.Admin.Port)
	url := "http://" + addr + "/admin"
	log.Printf("ailog admin → %s (allowed user: %s)", url, site.Admin.User)
	go openBrowser(url)
	return http.ListenAndServe(addr, mux)
}

var adminFuncs = template.FuncMap{
	"formatDate": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02")
	},
	"year": func() int { return time.Now().Year() },
}

// loadAdminPage parses _base.html + the named page (e.g. "list", "edit") in a
// fresh template namespace, so {{define "main"}} across pages doesn't collide.
// Pages execute as "_base".
func loadAdminPage(site *config.Site, name string) (*template.Template, error) {
	t := template.New("").Funcs(adminFuncs)
	files := []string{
		filepath.Join(site.ThemeDir(), "admin", "_base.html"),
		filepath.Join(site.ThemeDir(), "admin", name+".html"),
	}
	t, err := t.ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("parse admin page %s: %w", name, err)
	}
	return t, nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return
	}
	_ = cmd.Start()
}
