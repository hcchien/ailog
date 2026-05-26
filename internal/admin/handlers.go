package admin

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hcchien/ailog/internal/auth"
	"github.com/hcchien/ailog/internal/content"
	"github.com/hcchien/ailog/internal/gitops"
)

// routes wires up the admin HTTP handlers on the given mux.
func (s *Server) routes(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusFound)
	})
	mux.HandleFunc("/admin", s.requireAuthOrLogin(s.handleList))
	mux.HandleFunc("/admin/login", s.handleLogin)
	mux.HandleFunc("/admin/logout", s.handleLogout)
	mux.HandleFunc("/admin/new", s.requireAuth(s.handleNew))
	mux.HandleFunc("/admin/edit", s.requireAuth(s.handleEdit))
	mux.HandleFunc("/admin/save", s.requireAuth(s.handleSave))
	mux.HandleFunc("/admin/delete", s.requireAuth(s.handleDelete))
	mux.HandleFunc("/admin/preview", s.requireAuth(s.handlePreview))
	mux.HandleFunc("/admin/upload", s.requireAuth(s.handleUpload))

	// Serve uploaded images from public/ so the editor preview can resolve them.
	publicFS := http.FileServer(http.Dir(s.site.PublicDir()))
	mux.Handle("/images/", publicFS)

	// Serve the active theme's static/ directory at /theme/* so admin
	// templates can pull in the same CSS the public site uses.
	themeStatic := http.FileServer(http.Dir(filepath.Join(s.site.ThemeDir(), "static")))
	mux.Handle("/theme/", http.StripPrefix("/theme/", themeStatic))
}

// requireAuth blocks unauthenticated requests with 401 (used for non-page endpoints).
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.session(r) == nil {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// requireAuthOrLogin redirects unauthenticated requests to the login page (for navigations).
func (s *Server) requireAuthOrLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.session(r) == nil {
			s.renderLogin(w, "")
			return
		}
		next(w, r)
	}
}

func (s *Server) session(r *http.Request) *auth.Session {
	c, err := r.Cookie(auth.CookieName)
	if err != nil {
		return nil
	}
	return s.auth.Verify(c.Value)
}

// ---- login ----------------------------------------------------------------

func (s *Server) renderLogin(w http.ResponseWriter, errMsg string) {
	t, err := loadAdminPage(s.site, "login")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := map[string]any{
		"Site":  s.site,
		"Title": "Sign in",
		"Error": errMsg,
	}
	if err := t.ExecuteTemplate(w, "_base", data); err != nil {
		log.Printf("render login: %v", err)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.renderLogin(w, "")
		return
	}
	id, _, err := s.auth.Login()
	if err != nil {
		s.renderLogin(w, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(7 * 24 * time.Hour),
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.CookieName); err == nil {
		s.auth.Logout(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   auth.CookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// ---- list -----------------------------------------------------------------

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	posts, err := content.LoadAll(s.site.PostsDir())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	t, err := loadAdminPage(s.site, "list")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := map[string]any{
		"Site":   s.site,
		"Title":  "Posts",
		"User":   s.session(r).User,
		"Active": "list",
		"Posts":  posts,
	}
	if err := t.ExecuteTemplate(w, "_base", data); err != nil {
		log.Printf("render list: %v", err)
	}
}

// ---- new / edit -----------------------------------------------------------

func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	p := &content.Post{
		Frontmatter: content.Frontmatter{
			Title: "",
			Date:  time.Now(),
			Draft: true,
		},
		Slug: "",
		Body: "",
	}
	s.renderEdit(w, r, p, "")
}

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Error(w, "missing slug", 400)
		return
	}
	path := filepath.Join(s.site.PostsDir(), slug+".md")
	p, err := content.Load(path)
	if err != nil {
		http.Error(w, err.Error(), 404)
		return
	}
	s.renderEdit(w, r, p, "")
}

func (s *Server) renderEdit(w http.ResponseWriter, r *http.Request, p *content.Post, msg string) {
	all, _ := content.LoadAll(s.site.PostsDir())
	tagSet := map[string]struct{}{}
	for _, post := range all {
		for _, t := range post.Tags {
			tagSet[t] = struct{}{}
		}
	}
	var allTags []string
	for t := range tagSet {
		allTags = append(allTags, t)
	}
	sort.Strings(allTags)

	t, err := loadAdminPage(s.site, "edit")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	data := map[string]any{
		"Site":    s.site,
		"Title":   firstNonEmpty(p.Title, "New post"),
		"User":    s.session(r).User,
		"Active":  "edit",
		"Post":    p,
		"AllTags": allTags,
		"TagsCSV": strings.Join(p.Tags, ", "),
		"Message": msg,
		"IsNew":   p.Slug == "",
	}
	if err := t.ExecuteTemplate(w, "_base", data); err != nil {
		log.Printf("render edit: %v", err)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ---- save -----------------------------------------------------------------

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	dateStr := strings.TrimSpace(r.FormValue("date"))
	tagsCSV := r.FormValue("tags")
	desc := strings.TrimSpace(r.FormValue("description"))
	draft := r.FormValue("draft") == "on"
	feature := r.FormValue("feature") == "on"
	body := r.FormValue("body")
	slug := strings.TrimSpace(r.FormValue("slug"))
	originalSlug := strings.TrimSpace(r.FormValue("original_slug"))

	if title == "" {
		http.Error(w, "title is required", 400)
		return
	}
	date, err := parseDate(dateStr)
	if err != nil {
		http.Error(w, "bad date: "+err.Error(), 400)
		return
	}
	if slug == "" {
		slug = content.Slugify(title)
	}

	fm := content.Frontmatter{
		Title:       title,
		Date:        date,
		Tags:        parseTagsCSV(tagsCSV),
		Description: desc,
		Draft:       draft,
		Feature:     feature,
	}
	out, err := content.Serialize(fm, body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	newPath := filepath.Join(s.site.PostsDir(), slug+".md")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// If slug changed, remove the old file (so git sees a rename).
	if originalSlug != "" && originalSlug != slug {
		oldPath := filepath.Join(s.site.PostsDir(), originalSlug+".md")
		_ = os.Remove(oldPath)
	}
	if err := os.WriteFile(newPath, []byte(out), 0o644); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	commitMsg := fmt.Sprintf("post: %s", title)
	if err := s.commitAndPush(commitMsg); err != nil && err != gitops.ErrNoChanges {
		log.Printf("git after save: %v", err)
	}

	http.Redirect(w, r, "/admin/edit?slug="+slug, http.StatusSeeOther)
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	// HTML datetime-local: "2026-05-26T15:04"
	if t, err := time.ParseInLocation("2006-01-02T15:04", s, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("expected YYYY-MM-DD[THH:MM], got %q", s)
}

func parseTagsCSV(csv string) []string {
	var out []string
	for _, raw := range strings.Split(csv, ",") {
		t := strings.TrimSpace(raw)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (s *Server) commitAndPush(message string) error {
	if !s.site.Admin.AutoPush {
		return nil
	}
	repo := s.site.Root
	// Stage everything the admin can modify; only include paths that exist
	// on disk so a fresh repo without uploads doesn't fail.
	var paths []string
	for _, p := range []string{"content", "public"} {
		if _, err := os.Stat(filepath.Join(repo, p)); err == nil {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return gitops.ErrNoChanges
	}
	if err := gitops.AddCommit(repo, message, paths...); err != nil {
		return err
	}
	if !gitops.HasRemote(repo) {
		log.Printf("git: no remote configured, skipping push")
		return nil
	}
	if err := gitops.Push(repo); err != nil {
		return err
	}
	return nil
}

// ---- delete ---------------------------------------------------------------

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	slug := r.FormValue("slug")
	if slug == "" {
		http.Error(w, "missing slug", 400)
		return
	}
	path := filepath.Join(s.site.PostsDir(), slug+".md")
	if err := os.Remove(path); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := s.commitAndPush("post: delete " + slug); err != nil && err != gitops.ErrNoChanges {
		log.Printf("git after delete: %v", err)
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// ---- preview --------------------------------------------------------------

// handlePreview renders the markdown body via goldmark, so the editor preview
// matches what the public site will show.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	html, err := s.md.Render(string(body))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, string(html))
}

// ---- upload ---------------------------------------------------------------

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !validImageExt(ext) {
		http.Error(w, "unsupported file type", 400)
		return
	}

	now := time.Now()
	dir := filepath.Join(s.site.UploadsDir(), now.Format("2006/01"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	name := fmt.Sprintf("%d-%s", now.Unix(), sanitizeFilename(header.Filename))
	outPath := filepath.Join(dir, name)
	out, err := os.Create(outPath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	rel, _ := filepath.Rel(s.site.Root, outPath)
	urlPath := "/" + strings.TrimPrefix(strings.ReplaceAll(rel, string(os.PathSeparator), "/"), "public/")

	if err := s.commitAndPush("upload: " + name); err != nil && err != gitops.ErrNoChanges {
		log.Printf("git after upload: %v", err)
	}

	// EasyMDE expects JSON: {data: {filePath: ...}}
	fmt.Fprintf(w, `{"data":{"filePath":%q}}`, urlPath)
}

func validImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		return true
	}
	return false
}

var unsafeFilenameRe = func() func(string) string {
	return func(s string) string {
		var b strings.Builder
		for _, r := range s {
			switch {
			case r >= 'a' && r <= 'z',
				r >= 'A' && r <= 'Z',
				r >= '0' && r <= '9',
				r == '.', r == '-', r == '_':
				b.WriteRune(r)
			default:
				b.WriteRune('-')
			}
		}
		return b.String()
	}
}()

func sanitizeFilename(name string) string {
	return unsafeFilenameRe(name)
}

// keep imports minimal; suppress unused warnings if any field set later.
var _ = bytes.NewBuffer
var _ template.HTML
