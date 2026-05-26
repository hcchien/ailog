package build

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/feeds"

	"github.com/hcchien/ailog/internal/config"
	"github.com/hcchien/ailog/internal/content"
)

type pageData struct {
	Site    *config.Site
	Post    *content.Post
	Posts   []*content.Post
	Tag     string
	Tags    []tagEntry
	Title   string
	BuildAt time.Time
}

type tagEntry struct {
	Name  string
	Count int
}

// Run produces the full static site under site.DistDir().
func Run(site *config.Site) error {
	posts, err := content.LoadAll(site.PostsDir())
	if err != nil {
		return fmt.Errorf("load posts: %w", err)
	}
	published := content.Published(posts)

	md := Markdown()
	for _, p := range published {
		html, err := RenderMarkdown(md, p.Body)
		if err != nil {
			return fmt.Errorf("render %s: %w", p.Slug, err)
		}
		p.HTML = html
	}

	dist := site.DistDir()
	if err := os.RemoveAll(dist); err != nil {
		return err
	}
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}

	if err := renderHome(site, published, dist); err != nil {
		return err
	}
	if err := renderPosts(site, published, dist); err != nil {
		return err
	}
	if err := renderTags(site, published, dist); err != nil {
		return err
	}
	if err := renderRSS(site, published, dist); err != nil {
		return err
	}
	if err := render404(site, dist); err != nil {
		return err
	}

	if err := copyTree(filepath.Join(site.ThemeDir(), "static"), dist); err != nil {
		return fmt.Errorf("copy theme static: %w", err)
	}
	if err := copyTree(site.PublicDir(), dist); err != nil {
		return fmt.Errorf("copy public: %w", err)
	}

	if site.CustomDomain != "" {
		if err := os.WriteFile(filepath.Join(dist, "CNAME"), []byte(site.CustomDomain+"\n"), 0o644); err != nil {
			return err
		}
	}
	// .nojekyll prevents GitHub Pages from running Jekyll over our output.
	if err := os.WriteFile(filepath.Join(dist, ".nojekyll"), nil, 0o644); err != nil {
		return err
	}
	return nil
}

var funcMap = template.FuncMap{
	"formatDate": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("2006-01-02")
	},
	"year": func() int { return time.Now().Year() },
}

// loadPageTemplate parses _base.html + the named page layout in a fresh
// template namespace, so {{define "main"}} / {{define "head"}} from other
// pages can't bleed into this one.
func loadPageTemplate(site *config.Site, name string) (*template.Template, error) {
	t := template.New("").Funcs(funcMap)
	files := []string{
		filepath.Join(site.ThemeDir(), "layouts", "_base.html"),
		filepath.Join(site.ThemeDir(), "layouts", name+".html"),
	}
	t, err := t.ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	return t, nil
}

func renderHome(site *config.Site, posts []*content.Post, dist string) error {
	data := pageData{Site: site, Posts: posts, Title: "", BuildAt: time.Now()}
	return renderPage(site, "home", data, filepath.Join(dist, "index.html"))
}

func renderPosts(site *config.Site, posts []*content.Post, dist string) error {
	for _, p := range posts {
		data := pageData{Site: site, Post: p, Title: p.Title, BuildAt: time.Now()}
		out := filepath.Join(dist, "posts", p.Slug, "index.html")
		if err := renderPage(site, "post", data, out); err != nil {
			return fmt.Errorf("render post %s: %w", p.Slug, err)
		}
	}
	return nil
}

func renderTags(site *config.Site, posts []*content.Post, dist string) error {
	tagCounts := content.TagCount(posts)
	var tags []tagEntry
	for name, n := range tagCounts {
		tags = append(tags, tagEntry{Name: name, Count: n})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Name < tags[j].Name
	})
	indexData := pageData{Site: site, Tags: tags, Title: "Tags", BuildAt: time.Now()}
	if err := renderPage(site, "tags", indexData, filepath.Join(dist, "tags", "index.html")); err != nil {
		return err
	}
	for name := range tagCounts {
		tagged := content.ByTag(posts, name)
		data := pageData{Site: site, Posts: tagged, Tag: name, Title: "Tag: " + name, BuildAt: time.Now()}
		out := filepath.Join(dist, "tags", tagSlug(name), "index.html")
		if err := renderPage(site, "tag", data, out); err != nil {
			return fmt.Errorf("render tag %s: %w", name, err)
		}
	}
	return nil
}

func render404(site *config.Site, dist string) error {
	data := pageData{Site: site, Title: "Not found", BuildAt: time.Now()}
	return renderPage(site, "404", data, filepath.Join(dist, "404.html"))
}

func renderPage(site *config.Site, name string, data pageData, outPath string) error {
	t, err := loadPageTemplate(site, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "_base", data); err != nil {
		return fmt.Errorf("execute %s: %w", name, err)
	}
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

func renderRSS(site *config.Site, posts []*content.Post, dist string) error {
	feed := &feeds.Feed{
		Title:       site.Title,
		Link:        &feeds.Link{Href: site.BaseURL},
		Description: site.Description,
		Author:      &feeds.Author{Name: site.Author},
		Created:     time.Now(),
	}
	for _, p := range posts {
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       p.Title,
			Link:        &feeds.Link{Href: strings.TrimRight(site.BaseURL, "/") + p.URL()},
			Description: p.Description,
			Content:     string(p.HTML),
			Created:     p.Date,
		})
	}
	rss, err := feed.ToRss()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dist, "rss.xml"), []byte(rss), 0o644)
}

// tagSlug — keep tag URLs predictable.
func tagSlug(tag string) string {
	return strings.ToLower(strings.ReplaceAll(tag, " ", "-"))
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
