// Package access owns durable source permissions and revocable reader grants.
// It is deliberately separate from the expendable document cache.
package access

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/RSS-Can/internal/source"
	"golang.org/x/sys/unix"
)

var (
	ErrDenied     = errors.New("source access denied")
	ErrInvalid    = errors.New("invalid access record")
	ErrGrantLimit = errors.New("reader credential limit reached")
)

const maxStoreBytes = 8 << 20
const maxRecords = 10000

// MaxGrantsPerSubject bounds one account's active reader credentials across
// every source. Reserve global records for principals and source administration.
const MaxGrantsPerSubject = 64
const maxGrantRecords = 8192

type Principal struct {
	Disabled bool `json:"disabled"`
}

type Source struct {
	ID       string        `json:"id"`
	Owner    string        `json:"owner"`
	Public   bool          `json:"public"`
	Readers  []string      `json:"readers,omitempty"`
	Spec     *source.Spec  `json:"spec,omitempty"`
	Previous []source.Spec `json:"previous,omitempty"`
	Revision uint64        `json:"revision,omitempty"`
}

type Grant struct {
	Label     string    `json:"label,omitempty"`
	ID        string    `json:"id"`
	Source    string    `json:"source"`
	Subject   string    `json:"subject"`
	Digest    string    `json:"digest,omitempty"`
	Formats   []string  `json:"formats"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

type state struct {
	Issuer     string               `json:"issuer,omitempty"`
	Version    uint64               `json:"version"`
	Principals map[string]Principal `json:"principals"`
	Sources    map[string]Source    `json:"sources"`
	Grants     map[string]Grant     `json:"grants"`
}

// Store is single-process durable storage. An advisory lock prevents two instances
// from accepting conflicting writes; clustered deployments need a shared DB.
type Store struct {
	mu     sync.RWMutex
	path   string
	lock   *os.File
	state  state
	failed bool
}

func validID(s string) bool {
	if s == "" || len(s) > 256 || strings.TrimSpace(s) != s {
		return false
	}
	for _, r := range s {
		if r <= 32 || r == 127 || r == ',' || r == '/' || r == '\\' {
			return false
		}
	}
	return true
}

func Open(path string) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("access store must use an absolute path")
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if !dirInfo.IsDir() || dirInfo.Mode().Perm()&0077 != 0 {
		return nil, errors.New("access store directory must be private (0700)")
	}
	lock, err := os.OpenFile(path+".lock", os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("lock access store: %w", err)
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("access store is already in use: %w", err)
	}
	s := &Store{path: path, lock: lock, state: state{Version: 1, Principals: map[string]Principal{}, Sources: map[string]Source{}, Grants: map[string]Grant{}}}
	ok := false
	defer func() {
		if !ok {
			_ = s.Close()
		}
	}()
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := s.persist(s.state); err != nil {
			return nil, err
		}
	} else {
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > maxStoreBytes {
			return nil, fmt.Errorf("access store must be a private regular file within size limit")
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		decoder := json.NewDecoder(io.LimitReader(f, maxStoreBytes+1))
		decoder.DisallowUnknownFields()
		var loaded state
		if err = decoder.Decode(&loaded); err != nil {
			return nil, err
		}
		var extra any
		if err = decoder.Decode(&extra); err != io.EOF {
			return nil, ErrInvalid
		}
		if err = validate(loaded); err != nil {
			return nil, err
		}
		s.state = loaded
	}
	ok = true
	return s, nil
}

func validate(st state) error {
	if st.Version == 0 || st.Principals == nil || st.Sources == nil || st.Grants == nil || len(st.Principals)+len(st.Sources)+len(st.Grants) > maxRecords {
		return ErrInvalid
	}
	for id := range st.Principals {
		if !validID(id) {
			return ErrInvalid
		}
	}
	for id, src := range st.Sources {
		if id != src.ID || !validID(id) || !validID(src.Owner) || len(src.Readers) > 256 {
			return ErrInvalid
		}
		if _, ok := st.Principals[src.Owner]; !ok {
			return ErrInvalid
		}
		for _, r := range src.Readers {
			if !validID(r) {
				return ErrInvalid
			}
			if _, ok := st.Principals[r]; !ok {
				return ErrInvalid
			}
		}
		if len(src.Previous) > 5 {
			return ErrInvalid
		}
		if src.Spec != nil {
			if src.Revision == 0 || src.Spec.Validate() != nil {
				return ErrInvalid
			}
		}
		for _, old := range src.Previous {
			if old.Validate() != nil {
				return ErrInvalid
			}
		}
	}
	if err := managedCapacity(st); err != nil {
		return err
	}
	for id, g := range st.Grants {
		if len(g.Label) > 80 || strings.ContainsAny(g.Label, "\r\n\t") {
			return ErrInvalid
		}
		if id != g.ID || !validID(id) || !validID(g.Subject) || len(g.Digest) != 64 || g.ExpiresAt.IsZero() || !validFormats(g.Formats) {
			return ErrInvalid
		}
		if _, err := hex.DecodeString(g.Digest); err != nil {
			return ErrInvalid
		}
		if _, ok := st.Sources[g.Source]; !ok {
			return ErrInvalid
		}
		if _, ok := st.Principals[g.Subject]; !ok {
			return ErrInvalid
		}
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil
	}
	err := s.lock.Close()
	s.lock = nil
	return err
}

func (s *Store) persist(st state) error {
	if err := validate(st); err != nil {
		return err
	}
	body, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if len(body) > maxStoreBytes {
		return ErrInvalid
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".access-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(body)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, s.path); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	err = d.Sync()
	return errors.Join(err, d.Close())
}

func (s *Store) update(fn func(*state) error) error {
	return s.updateState(true, fn)
}

// Credential changes are authenticated afresh on every request and do not
// change source ACLs. Do not invalidate every tenant's content cache for them.
func (s *Store) updateState(permissionsChanged bool, fn func(*state) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil || s.failed {
		return errors.New("access store is closed")
	}
	body, _ := json.Marshal(s.state)
	var next state
	_ = json.Unmarshal(body, &next)
	pruneInactiveGrants(&next, time.Now())
	if err := fn(&next); err != nil {
		return err
	}
	if permissionsChanged {
		next.Version++
	}
	if err := s.persist(next); err != nil {
		// A failed durability operation must not leave the process serving old
		// permissions after a potentially committed revoke. Restart/recovery is
		// required; invalid requested records have not touched the disk.
		if !errors.Is(err, ErrInvalid) {
			s.failed = true
		}
		return err
	}
	s.state = next
	return nil
}

func (s *Store) SetPrincipal(subject string, disabled bool) error {
	subject = strings.Clone(subject)
	if !validID(subject) {
		return ErrInvalid
	}
	return s.update(func(st *state) error {
		st.Principals[subject] = Principal{Disabled: disabled}
		if disabled {
			for id, g := range st.Grants {
				if g.Subject == subject {
					g.Revoked = true
					st.Grants[id] = g
				}
			}
		}
		return nil
	})
}

// BindIssuer prevents accidental identity rebinding when a deployment points
// an existing permission database at a different directory authority.
func (s *Store) BindIssuer(issuer string) error {
	issuer = strings.Clone(issuer)
	if issuer == "" {
		return ErrInvalid
	}
	return s.update(func(st *state) error {
		if st.Issuer != "" && st.Issuer != issuer {
			return errors.New("access store belongs to a different directory issuer")
		}
		st.Issuer = issuer
		return nil
	})
}

func (s *Store) PutSource(src Source) error {
	src.Readers = append([]string(nil), src.Readers...)
	src = cloneSource(src)
	if !validID(src.ID) || !validID(src.Owner) || len(src.Readers) > 256 {
		return ErrInvalid
	}
	for _, r := range src.Readers {
		if !validID(r) {
			return ErrInvalid
		}
	}
	return s.update(func(st *state) error {
		// Moving ownership revokes all outstanding grants, including grants
		// belonging to readers who might also be present in the new ACL.
		if old, ok := st.Sources[src.ID]; ok && old.Owner != src.Owner {
			for id, g := range st.Grants {
				if g.Source == src.ID {
					g.Revoked = true
					st.Grants[id] = g
				}
			}
		}
		if _, ok := st.Principals[src.Owner]; !ok {
			st.Principals[src.Owner] = Principal{}
		}
		for _, r := range src.Readers {
			if _, ok := st.Principals[r]; !ok {
				st.Principals[r] = Principal{}
			}
		}
		st.Sources[src.ID] = src
		for id, g := range st.Grants {
			if g.Source == src.ID && !canRead(*st, g.Subject, src.ID) {
				g.Revoked = true
				st.Grants[id] = g
			}
		}
		return nil
	})
}

func canRead(st state, subject, source string) bool {
	src, ok := st.Sources[source]
	if !ok {
		return false
	}
	if src.Public && subject == "" {
		return true
	}
	p, ok := st.Principals[subject]
	if !ok || p.Disabled {
		return false
	}
	if src.Public || src.Owner == subject {
		return true
	}
	for _, r := range src.Readers {
		if r == subject {
			return true
		}
	}
	return false
}

func (s *Store) CanRead(subject, source string) (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Version, s.lock != nil && !s.failed && canRead(s.state, subject, source)
}
func (s *Store) Active(subject string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.state.Principals[subject]
	return s.lock != nil && !s.failed && ok && !p.Disabled
}

func (s *Store) Sources(subject string) []Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Source
	if s.lock == nil || s.failed {
		return out
	}
	for id, src := range s.state.Sources {
		if canRead(s.state, subject, id) {
			out = append(out, cloneSource(src))
		}
	}
	return out
}

func validFormats(formats []string) bool {
	if len(formats) == 0 || len(formats) > 3 {
		return false
	}
	seen := map[string]bool{}
	for _, f := range formats {
		if (f != "rss" && f != "atom" && f != "json") || seen[f] {
			return false
		}
		seen[f] = true
	}
	return true
}
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreateGrant returns the secret exactly once. Only its SHA-256 digest persists.
func (s *Store) CreateGrant(subject, source string, formats []string, expires time.Time, labels ...string) (Grant, string, error) {
	subject, source = strings.Clone(subject), strings.Clone(source)
	formats = append([]string(nil), formats...)
	for i := range formats {
		formats[i] = strings.Clone(formats[i])
	}
	label := ""
	if len(labels) > 0 {
		label = strings.Clone(strings.TrimSpace(labels[0]))
		if len(label) > 80 || strings.ContainsAny(label, "\r\n\t") {
			return Grant{}, "", ErrInvalid
		}
	}
	if !validFormats(formats) || !expires.After(time.Now()) || expires.After(time.Now().Add(366*24*time.Hour)) {
		return Grant{}, "", ErrInvalid
	}
	now := time.Now()
	// Reject unauthorised and over-quota requests before cloning the full store.
	// The transaction repeats these checks to make concurrent issuance atomic.
	s.mu.RLock()
	var preflight error
	if s.lock == nil || s.failed {
		preflight = errors.New("access store is closed")
	} else if !canRead(s.state, subject, source) {
		preflight = ErrDenied
	} else {
		preflight = checkGrantQuota(s.state, subject, now)
	}
	s.mu.RUnlock()
	if preflight != nil {
		return Grant{}, "", preflight
	}
	id, err := randomToken(16)
	if err != nil {
		return Grant{}, "", err
	}
	secret, err := randomToken(32)
	if err != nil {
		return Grant{}, "", err
	}
	digest := sha256.Sum256([]byte(secret))
	g := Grant{ID: id, Source: source, Subject: subject, Digest: hex.EncodeToString(digest[:]), Formats: append([]string(nil), formats...), ExpiresAt: expires, Label: label}
	err = s.updateState(false, func(st *state) error {
		if !canRead(*st, subject, source) {
			return ErrDenied
		}
		now := time.Now()
		if err := checkGrantQuota(*st, subject, now); err != nil {
			return err
		}
		st.Grants[id] = g
		return nil
	})
	if err != nil {
		return Grant{}, "", err
	}
	g.Digest = ""
	g.Formats = append([]string(nil), g.Formats...)
	return g, secret, nil
}

func (s *Store) ResolveGrant(id, secret, format string, now time.Time) (Grant, uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.state.Grants[id]
	digest := sha256.Sum256([]byte(secret))
	expected, _ := hex.DecodeString(g.Digest)
	if !ok || len(secret) != 43 || subtle.ConstantTimeCompare(digest[:], expected) != 1 || g.Revoked || !now.Before(g.ExpiresAt) || s.lock == nil || s.failed || !canRead(s.state, g.Subject, g.Source) {
		return Grant{}, 0, ErrDenied
	}
	allowed := false
	for _, f := range g.Formats {
		allowed = allowed || format == f
	}
	if !allowed {
		return Grant{}, 0, ErrDenied
	}
	g.Digest = ""
	g.Formats = append([]string(nil), g.Formats...)
	return g, s.state.Version, nil
}

func (s *Store) RevokeGrant(subject, id string) error {
	return s.updateState(false, func(st *state) error {
		g, ok := st.Grants[id]
		if !ok || g.Subject != subject {
			return ErrDenied
		}
		g.Revoked = true
		st.Grants[id] = g
		return nil
	})
}

func checkGrantQuota(st state, subject string, now time.Time) error {
	active, owned := 0, 0
	for _, g := range st.Grants {
		if g.Revoked || !now.Before(g.ExpiresAt) {
			continue
		}
		active++
		if g.Subject == subject {
			owned++
		}
	}
	if owned >= MaxGrantsPerSubject || active >= maxGrantRecords {
		return ErrGrantLimit
	}
	return nil
}

// Reclaim inactive credentials during successful mutations, including operator
// changes, so historical grants cannot occupy reserved administration records.
func pruneInactiveGrants(st *state, now time.Time) {
	for id, g := range st.Grants {
		if g.Revoked || !now.Before(g.ExpiresAt) {
			delete(st.Grants, id)
		}
	}
}
