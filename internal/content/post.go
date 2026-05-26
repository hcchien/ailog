package content

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Cover struct {
	Hue   int    `yaml:"hue"`
	Label string `yaml:"label"`
}

type Frontmatter struct {
	Title       string    `yaml:"title"`
	Date        time.Time `yaml:"date"`
	Tags        []string  `yaml:"tags"`
	Description string    `yaml:"description"`
	Draft       bool      `yaml:"draft"`
	Feature     bool      `yaml:"feature"`
	ReadingMin  int       `yaml:"readingMin"`
	Cover       Cover     `yaml:"cover"`
}

type Post struct {
	Frontmatter
	Slug   string        // derived from filename
	Path   string        // absolute path on disk
	Body   string        // raw markdown (without frontmatter)
	HTML   template.HTML // rendered HTML (populated by build pipeline)
	Source string        // full file content including frontmatter
}

func (p *Post) URL() string {
	return "/posts/" + p.Slug + "/"
}

func (p *Post) DateString() string {
	if p.Date.IsZero() {
		return ""
	}
	return p.Date.Format("2006-01-02")
}

// CoverHue returns the explicit hue (0..360) or one derived deterministically
// from the slug — so every post gets a stable cover gradient.
func (p *Post) CoverHue() int {
	if p.Cover.Hue > 0 {
		return p.Cover.Hue
	}
	h := 0
	for _, r := range p.Slug {
		h = (h*131 + int(r)) % 360
	}
	if h < 0 {
		h += 360
	}
	// nudge into the teal/mint/blue band that all three themes assume
	return 140 + (h % 80)
}

// CoverLabel is the small text overlay on the gradient cover.
func (p *Post) CoverLabel() string {
	if p.Cover.Label != "" {
		return p.Cover.Label
	}
	return p.Slug
}

// ReadingMinutes returns the configured reading time or one derived from word
// count (~220 wpm).
func (p *Post) ReadingMinutes() int {
	if p.ReadingMin > 0 {
		return p.ReadingMin
	}
	words := len(strings.Fields(p.Body))
	if words == 0 {
		return 1
	}
	min := (words + 219) / 220
	if min < 1 {
		return 1
	}
	return min
}

// FirstTag returns the primary tag or "essay" as a fallback.
func (p *Post) FirstTag() string {
	if len(p.Tags) > 0 {
		return p.Tags[0]
	}
	return "essay"
}

var frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n?`)

// Parse extracts frontmatter and body from raw markdown text.
func Parse(raw string) (Frontmatter, string, error) {
	var fm Frontmatter
	m := frontmatterRe.FindStringSubmatchIndex(raw)
	if m == nil {
		return fm, raw, nil
	}
	yamlBlock := raw[m[2]:m[3]]
	body := raw[m[1]:]
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return fm, body, fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, body, nil
}

// Serialize writes frontmatter + body back to the on-disk format.
func Serialize(fm Frontmatter, body string) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(fm); err != nil {
		return "", err
	}
	enc.Close()
	out := "---\n" + buf.String() + "---\n\n" + strings.TrimLeft(body, "\n")
	return out, nil
}

// Load reads a single post from disk.
func Load(path string) (*Post, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fm, body, err := Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	slug := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return &Post{
		Frontmatter: fm,
		Slug:        slug,
		Path:        path,
		Body:        body,
		Source:      string(data),
	}, nil
}

// LoadAll reads every *.md file under dir, sorted by date desc.
func LoadAll(dir string) ([]*Post, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var posts []*Post
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].Date.After(posts[j].Date)
	})
	return posts, nil
}

// Published returns only non-draft posts.
func Published(all []*Post) []*Post {
	out := make([]*Post, 0, len(all))
	for _, p := range all {
		if !p.Draft {
			out = append(out, p)
		}
	}
	return out
}

// TagCount aggregates tag → post count from a slice of posts.
func TagCount(posts []*Post) map[string]int {
	out := map[string]int{}
	for _, p := range posts {
		for _, t := range p.Tags {
			out[t]++
		}
	}
	return out
}

// ByTag returns posts that include the given tag.
func ByTag(posts []*Post, tag string) []*Post {
	var out []*Post
	for _, p := range posts {
		for _, t := range p.Tags {
			if t == tag {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify turns a free-form title into a filename-safe slug.
func Slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return time.Now().Format("post-20060102-150405")
	}
	return s
}
