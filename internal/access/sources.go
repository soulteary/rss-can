package access

import (
	"encoding/json"
	"errors"
	"github.com/soulteary/RSS-Can/internal/source"
	"strings"
	"time"
)

func (s *Store) Grants(subject string) []Grant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Grant{}
	if s.lock == nil || s.failed {
		return out
	}
	for _, g := range s.state.Grants {
		if g.Subject == subject && !g.Revoked && time.Now().Before(g.ExpiresAt) {
			g.Digest = ""
			g.Formats = append([]string(nil), g.Formats...)
			out = append(out, g)
		}
	}
	return out
}

func cloneSource(src Source) Source {
	src.ID, src.Owner = strings.Clone(src.ID), strings.Clone(src.Owner)
	src.Readers = append([]string(nil), src.Readers...)
	for i := range src.Readers {
		src.Readers[i] = strings.Clone(src.Readers[i])
	}
	src.Previous = append([]source.Spec(nil), src.Previous...)
	for i := range src.Previous {
		src.Previous[i] = src.Previous[i].Clone()
	}
	if src.Spec != nil {
		spec := src.Spec.Clone()
		src.Spec = &spec
	}
	return src
}

// Managed returns a copy for the trusted application adapter, not an HTTP API.
func (s *Store) Managed(id string) (Source, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src, ok := s.state.Sources[id]
	return cloneSource(src), ok && src.Spec != nil && s.lock != nil && !s.failed
}
func (s *Store) ManagedSources() []Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Source
	if s.lock == nil || s.failed {
		return out
	}
	for _, src := range s.state.Sources {
		if src.Spec != nil {
			out = append(out, cloneSource(src))
		}
	}
	return out
}

func (s *Store) CreateManaged(subject string, spec source.Spec) (Source, error) {
	subject = strings.Clone(subject)
	spec = spec.Normalized()
	if spec.Validate() != nil {
		return Source{}, ErrInvalid
	}
	id, err := randomToken(16)
	if err != nil {
		return Source{}, err
	}
	src := Source{ID: "src_" + id, Owner: subject, Spec: &spec, Revision: 1}
	err = s.updateState(false, func(st *state) error {
		p, ok := st.Principals[subject]
		if !ok || p.Disabled {
			return ErrDenied
		}
		owned := 0
		for _, src := range st.Sources {
			if src.Owner == subject && src.Spec != nil {
				owned++
			}
		}
		if owned >= MaxManagedSourcesPerSubject {
			return ErrSourceLimit
		}
		st.Sources[src.ID] = src
		return managedCapacity(*st)
	})
	return cloneSource(src), err
}

func (s *Store) UpdateManaged(subject, id string, spec source.Spec, expected uint64) (Source, error) {
	spec = spec.Normalized()
	if spec.Validate() != nil {
		return Source{}, ErrInvalid
	}
	var result Source
	err := s.updateState(false, func(st *state) error {
		src, ok := st.Sources[id]
		p, known := st.Principals[subject]
		if !ok || src.Spec == nil || src.Owner != subject || !known || p.Disabled {
			return ErrDenied
		}
		if src.Revision != expected {
			return ErrConflict
		}
		src.Previous = append([]source.Spec{*src.Spec}, src.Previous...)
		if len(src.Previous) > 5 {
			src.Previous = src.Previous[:5]
		}
		src.Spec = &spec
		src.Revision++
		st.Sources[id] = src
		result = cloneSource(src)
		return managedCapacity(*st)
	})
	return result, err
}

var ErrConflict = errors.New("source version changed; reload before editing")
var ErrSourceLimit = errors.New("managed source limit reached")

const MaxManagedSourcesPerSubject = 100
const MaxManagedBytesPerSubject = 256 << 10
const MaxManagedBytesTotal = 4 << 20

func managedCapacity(st state) error {
	byOwner := map[string]int{}
	countByOwner := map[string]int{}
	total := 0
	for _, src := range st.Sources {
		if src.Spec == nil {
			continue
		}
		encoded, err := json.Marshal(src)
		if err != nil {
			return ErrInvalid
		}
		countByOwner[src.Owner]++
		if countByOwner[src.Owner] > MaxManagedSourcesPerSubject {
			return ErrSourceLimit
		}
		byOwner[src.Owner] += len(encoded)
		total += len(encoded)
		if byOwner[src.Owner] > MaxManagedBytesPerSubject || total > MaxManagedBytesTotal {
			return ErrSourceLimit
		}
	}
	return nil
}

func (s *Store) EnsurePrincipal(subject string) error {
	subject = strings.Clone(subject)
	if !validID(subject) {
		return ErrInvalid
	}
	s.mu.RLock()
	if s.lock == nil || s.failed {
		s.mu.RUnlock()
		return errors.New("authorization store unavailable")
	}
	p, exists := s.state.Principals[subject]
	s.mu.RUnlock()
	if exists {
		if p.Disabled {
			return ErrDenied
		}
		return nil
	}
	return s.updateState(false, func(st *state) error {
		if p, exists := st.Principals[subject]; exists {
			if p.Disabled {
				return ErrDenied
			}
			return nil
		}
		st.Principals[subject] = Principal{}
		return nil
	})
}

func (s *Store) RollbackManaged(subject, id string, expected uint64) (Source, error) {
	current, ok := s.Managed(id)
	if !ok || current.Owner != subject {
		return Source{}, ErrDenied
	}
	if len(current.Previous) == 0 {
		return Source{}, ErrInvalid
	}
	if current.Revision != expected {
		return Source{}, ErrConflict
	}
	return s.UpdateManaged(subject, id, current.Previous[0], expected)
}

func (s *Store) Source(id string) (Source, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src, ok := s.state.Sources[id]
	return cloneSource(src), ok && s.lock != nil && !s.failed
}

func (s *Store) DeleteManaged(subject, id string, expected uint64) error {
	return s.updateState(false, func(st *state) error {
		src, ok := st.Sources[id]
		p, known := st.Principals[subject]
		if !ok || src.Spec == nil || src.Owner != subject || !known || p.Disabled {
			return ErrDenied
		}
		if src.Revision != expected {
			return ErrConflict
		}
		delete(st.Sources, id)
		for gid, g := range st.Grants {
			if g.Source == id {
				delete(st.Grants, gid)
			}
		}
		return nil
	})
}
