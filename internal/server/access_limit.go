package server

import (
	"sync"
	"time"
)

const writeBurst = 8
const maxWriteSubjects = 4096

type writeBucket struct {
	tokens float64
	at     time.Time
}
type subjectWriteLimiter struct {
	mu      sync.Mutex
	entries map[string]writeBucket
}

func (l *subjectWriteLimiter) allow(subject string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = map[string]writeBucket{}
	}
	b, exists := l.entries[subject]
	if !exists {
		if len(l.entries) >= maxWriteSubjects {
			for key, entry := range l.entries {
				if now.Sub(entry.at) > 15*time.Minute {
					delete(l.entries, key)
				}
			}
		}
		if len(l.entries) >= maxWriteSubjects {
			return false
		}
		b = writeBucket{tokens: writeBurst, at: now}
	} else {
		b.tokens = min(writeBurst, b.tokens+max(0, now.Sub(b.at).Seconds())/5)
		b.at = now
	}
	if b.tokens < 1 {
		l.entries[subject] = b
		return false
	}
	b.tokens--
	l.entries[subject] = b
	return true
}
