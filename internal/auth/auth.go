package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// User is the GitHub identity returned by /user.
type User struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// Session is a single authenticated browser session.
type Session struct {
	User      User
	ExpiresAt time.Time
}

// Manager owns the session store and verifies GitHub identity via `gh`.
type Manager struct {
	allowedUser string
	sessions    map[string]Session
	mu          sync.Mutex
}

func NewManager(allowedUser string) *Manager {
	return &Manager{
		allowedUser: allowedUser,
		sessions:    map[string]Session{},
	}
}

// GHToken reads a token from the local `gh` CLI. Returns the token and the
// authenticated GitHub user.
func GHToken() (string, *User, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return "", nil, fmt.Errorf("gh CLI not found in PATH — install from https://cli.github.com/")
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", nil, fmt.Errorf("gh auth token failed — try `gh auth login`: %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", nil, fmt.Errorf("gh returned empty token — try `gh auth login`")
	}
	user, err := fetchUser(token)
	if err != nil {
		return token, nil, err
	}
	return token, user, nil
}

func fetchUser(token string) (*User, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch GitHub user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub /user returned %d: %s", resp.StatusCode, string(body))
	}
	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// Login verifies that gh's current user equals the allowed user, then issues
// a new session. Returns the session id (to set as a cookie) and the user.
func (m *Manager) Login() (string, *User, error) {
	_, user, err := GHToken()
	if err != nil {
		return "", nil, err
	}
	if !strings.EqualFold(user.Login, m.allowedUser) {
		return "", user, fmt.Errorf("only %q can log in (saw %q)", m.allowedUser, user.Login)
	}
	id := newSessionID()
	m.mu.Lock()
	m.sessions[id] = Session{
		User:      *user,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}
	m.mu.Unlock()
	return id, user, nil
}

// Verify returns the session for the cookie value, or nil if missing/expired.
func (m *Manager) Verify(id string) *Session {
	if id == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil
	}
	if time.Now().After(s.ExpiresAt) {
		delete(m.sessions, id)
		return nil
	}
	return &s
}

// Logout removes the session.
func (m *Manager) Logout(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func newSessionID() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

const CookieName = "ailog_session"
