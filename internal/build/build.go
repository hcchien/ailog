package build

import (
	"bytes"
	"encoding/json"
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

// buildSearchIndex serialises the published posts into a small JSON array
// that themes can drop into a <script> tag for client-side ⌘K search.
func buildSearchIndex(site *config.Site, posts []*content.Post) template.JS {
	type entry struct {
		Title       string   `json:"title"`
		URL         string   `json:"url"`
		Slug        string   `json:"slug"`
		Description string   `json:"description"`
		Date        string   `json:"date"`
		Tags        []string `json:"tags"`
		ReadingMin  int      `json:"readingMin"`
	}
	out := make([]entry, 0, len(posts))
	for _, p := range posts {
		out = append(out, entry{
			Title:       p.Title,
			URL:         site.URL(p.URL()),
			Slug:        p.Slug,
			Description: p.Description,
			Date:        p.Date.Format("2006-01-02"),
			Tags:        p.Tags,
			ReadingMin:  p.ReadingMinutes(),
		})
	}
	b, _ := json.Marshal(out)
	return template.JS(b)
}

type pageData struct {
	Site        *config.Site
	Post        *content.Post
	Prev        *content.Post
	Next        *content.Post
	Featured    *content.Post   // only on home
	Posts       []*content.Post
	AllPosts    []*content.Post // every published post, regardless of filter
	Tag         string
	Tags        []TagEntry
	Title       string
	Page        string        // "home", "post", "tag", "tags", "404"
	PostIndex   int           // 0-based position of Post in AllPosts (newest = 0)
	SearchIndex template.JS   // serialized post index for client-side search
	BuildAt     time.Time
}

// TagEntry is exported so templates can sort/iterate.
type TagEntry struct {
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

// loadPageTemplate parses _base.html + the named page layout in a fresh
// template namespace, so {{define "main"}} / {{define "head"}} from other
// pages can't bleed into this one.
func loadPageTemplate(site *config.Site, name string) (*template.Template, error) {
	funcMap := template.FuncMap{
		"formatDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"longDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("January 2, 2006")
		},
		"shortDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("Jan 02")
		},
		"isoDate": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"relTime": relTimeFunc,
		"pad2":    func(n int) string { return fmt.Sprintf("%02d", n) },
		"year":    func() int { return time.Now().Year() },
		"now":     func() time.Time { return time.Now() },
		// {{url "/style.css"}} → "/ailog/style.css" under a basePath.
		"url": site.URL,
		// {{tagURL "AI"}} → "/ailog/tags/ai/"
		"tagURL": func(tag string) string {
			return site.URL("/tags/" + tagSlug(tag) + "/")
		},
		// {{coverSVG hue label}} → inline SVG gradient cover for the given post.
		"coverSVG": coverSVG,
		// CoverHue/ReadingMinutes wrappers so templates can call without method chains.
		"coverHue": func(p *content.Post) int { return p.CoverHue() },
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		// Look up a post by slug — handy for prev/next titles when posts are not in scope.
		"safeHTML": func(s string) template.HTML { return template.HTML(s) },
	}
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
	var featured *content.Post
	for _, p := range posts {
		if p.Feature {
			featured = p
			break
		}
	}
	if featured == nil && len(posts) > 0 {
		featured = posts[0]
	}
	rest := make([]*content.Post, 0, len(posts))
	for _, p := range posts {
		if featured != nil && p.Slug == featured.Slug {
			continue
		}
		rest = append(rest, p)
	}
	data := pageData{
		Site:        site,
		Featured:    featured,
		Posts:       rest,
		AllPosts:    posts,
		Tags:        allTagEntries(posts),
		Page:        "home",
		Title:       "",
		SearchIndex: buildSearchIndex(site, posts),
		BuildAt:     time.Now(),
	}
	return renderPage(site, "home", data, filepath.Join(dist, "index.html"))
}

func renderPosts(site *config.Site, posts []*content.Post, dist string) error {
	// posts are sorted by date desc. The themes follow the design package's
	// convention: Prev = the previously-published (more recent) essay shown
	// to the left of the current article, Next = the older essay to the right.
	for i, p := range posts {
		var prev, next *content.Post
		if i-1 >= 0 {
			prev = posts[i-1]
		}
		if i+1 < len(posts) {
			next = posts[i+1]
		}
		data := pageData{
			Site:        site,
			Post:        p,
			Prev:        prev,
			Next:        next,
			AllPosts:    posts,
			Tags:        allTagEntries(posts),
			Page:        "post",
			Title:       p.Title,
			PostIndex:   i,
			SearchIndex: buildSearchIndex(site, posts),
			BuildAt:     time.Now(),
		}
		out := filepath.Join(dist, "posts", p.Slug, "index.html")
		if err := renderPage(site, "post", data, out); err != nil {
			return fmt.Errorf("render post %s: %w", p.Slug, err)
		}
	}
	return nil
}

func allTagEntries(posts []*content.Post) []TagEntry {
	counts := content.TagCount(posts)
	tags := make([]TagEntry, 0, len(counts))
	for name, n := range counts {
		tags = append(tags, TagEntry{Name: name, Count: n})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Name < tags[j].Name
	})
	return tags
}

func renderTags(site *config.Site, posts []*content.Post, dist string) error {
	tags := allTagEntries(posts)
	searchIdx := buildSearchIndex(site, posts)
	indexData := pageData{
		Site:        site,
		Tags:        tags,
		AllPosts:    posts,
		Page:        "tags",
		Title:       "Tags",
		SearchIndex: searchIdx,
		BuildAt:     time.Now(),
	}
	if err := renderPage(site, "tags", indexData, filepath.Join(dist, "tags", "index.html")); err != nil {
		return err
	}
	for _, t := range tags {
		tagged := content.ByTag(posts, t.Name)
		data := pageData{
			Site:        site,
			Posts:       tagged,
			AllPosts:    posts,
			Tags:        tags,
			Tag:         t.Name,
			Page:        "tag",
			Title:       "#" + t.Name,
			SearchIndex: searchIdx,
			BuildAt:     time.Now(),
		}
		out := filepath.Join(dist, "tags", tagSlug(t.Name), "index.html")
		if err := renderPage(site, "tag", data, out); err != nil {
			return fmt.Errorf("render tag %s: %w", t.Name, err)
		}
	}
	return nil
}

func render404(site *config.Site, dist string) error {
	posts, _ := content.LoadAll(site.PostsDir())
	pub := content.Published(posts)
	data := pageData{
		Site:        site,
		AllPosts:    pub,
		Tags:        allTagEntries(pub),
		Page:        "404",
		Title:       "Not found",
		SearchIndex: buildSearchIndex(site, pub),
		BuildAt:     time.Now(),
	}
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
		Link:        &feeds.Link{Href: site.AbsURL("/")},
		Description: site.Description,
		Author:      &feeds.Author{Name: site.Author},
		Created:     time.Now(),
	}
	for _, p := range posts {
		feed.Items = append(feed.Items, &feeds.Item{
			Title:       p.Title,
			Link:        &feeds.Link{Href: site.AbsURL(p.URL())},
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

// relTimeFunc returns a short relative time string ("3d ago", "2w ago").
func relTimeFunc(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	days := int(d.Hours() / 24)
	if days < 0 {
		days = -days
	}
	switch {
	case days < 1:
		return "today"
	case days < 7:
		return fmt.Sprintf("%dd ago", days)
	case days < 30:
		return fmt.Sprintf("%dw ago", days/7)
	case days < 365:
		return fmt.Sprintf("%dmo ago", days/30)
	default:
		return fmt.Sprintf("%dy ago", days/365)
	}
}

// coverSVG renders the gradient cover used by every theme. The themes' own
// CSS handles the framing; this function just emits the inline gradient.
func coverSVG(hue int, label string) template.HTML {
	h := ((hue % 360) + 360) % 360
	a := fmt.Sprintf("oklch(0.82 0.10 %d)", h)
	b := fmt.Sprintf("oklch(0.55 0.10 %d)", (h+18)%360)
	c := fmt.Sprintf("oklch(0.35 0.08 %d)", ((h-12)%360+360)%360)
	id := fmt.Sprintf("cv%d", h)
	svg := fmt.Sprintf(`<svg class="cover-svg" width="100%%" height="100%%" viewBox="0 0 600 480" preserveAspectRatio="xMidYMid slice" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
<defs>
<linearGradient id="g-%s" x1="0" y1="0" x2="1" y2="1">
<stop offset="0%%" stop-color="%s"/>
<stop offset="55%%" stop-color="%s"/>
<stop offset="100%%" stop-color="%s"/>
</linearGradient>
<pattern id="p-%s" width="24" height="24" patternUnits="userSpaceOnUse">
<path d="M 24 0 L 0 0 0 24" fill="none" stroke="rgba(255,255,255,0.06)" stroke-width="1"/>
</pattern>
</defs>
<rect width="600" height="480" fill="url(#g-%s)"/>
<rect width="600" height="480" fill="url(#p-%s)"/>
<g style="mix-blend-mode:overlay">
<circle cx="120" cy="380" r="180" fill="%s" opacity="0.4"/>
<circle cx="510" cy="80" r="100" fill="%s" opacity="0.5"/>
</g>
<text x="20" y="30" fill="rgba(255,255,255,0.78)" font-family="ui-monospace, monospace" font-size="11" letter-spacing="2">%s</text>
</svg>`, id, a, b, c, id, id, id, a, c, template.HTMLEscapeString(label))
	return template.HTML(svg)
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
