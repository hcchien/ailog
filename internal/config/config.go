package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type AdminConfig struct {
	User       string `yaml:"user"`
	Port       int    `yaml:"port"`
	PostsDir   string `yaml:"postsDir"`
	UploadsDir string `yaml:"uploadsDir"`
	AutoPush   bool   `yaml:"autoPush"`
}

type Site struct {
	Title        string      `yaml:"title"`
	Description  string      `yaml:"description"`
	Author       string      `yaml:"author"`
	BaseURL      string      `yaml:"baseURL"`
	Language     string      `yaml:"language"`
	CustomDomain string      `yaml:"customDomain"`
	PostsPerPage int         `yaml:"postsPerPage"`
	Admin        AdminConfig `yaml:"admin"`

	Root string `yaml:"-"`
}

func Load(path string) (*Site, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", abs, err)
	}
	var s Site
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", abs, err)
	}
	s.Root = filepath.Dir(abs)
	s.applyDefaults()
	return &s, nil
}

func (s *Site) applyDefaults() {
	if s.Language == "" {
		s.Language = "zh-Hant"
	}
	if s.PostsPerPage == 0 {
		s.PostsPerPage = 20
	}
	if s.Admin.Port == 0 {
		s.Admin.Port = 8080
	}
	if s.Admin.PostsDir == "" {
		s.Admin.PostsDir = "content/posts"
	}
	if s.Admin.UploadsDir == "" {
		s.Admin.UploadsDir = "public/images"
	}
}

func (s *Site) PostsDir() string   { return filepath.Join(s.Root, s.Admin.PostsDir) }
func (s *Site) UploadsDir() string { return filepath.Join(s.Root, s.Admin.UploadsDir) }
func (s *Site) PublicDir() string  { return filepath.Join(s.Root, "public") }
func (s *Site) ThemeDir() string   { return filepath.Join(s.Root, "themes", "default") }
func (s *Site) DistDir() string    { return filepath.Join(s.Root, "dist") }
