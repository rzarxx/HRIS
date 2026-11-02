package auth

import (
	"sync"
	"time"
)

// Limiter menghitung permintaan per kunci. Implementasi Redis menggantikan MemoryLimiter
// begitu API berjalan lebih dari satu instance (MASTER.md §5.7).
type Limiter interface {
	// Allow mengembalikan false beserta sisa waktu tunggu bila batas terlampaui.
	Allow(key string, limit int, window time.Duration, now time.Time) (bool, time.Duration)
	// Reset menghapus hitungan, mis. setelah login berhasil.
	Reset(key string)
}

type windowCount struct {
	count   int
	resetAt time.Time
}

// MemoryLimiter memakai fixed window di memori proses.
type MemoryLimiter struct {
	mu        sync.Mutex
	counts    map[string]windowCount
	lastSweep time.Time
}

func NewMemoryLimiter() *MemoryLimiter {
	return &MemoryLimiter{counts: map[string]windowCount{}}
}

func (l *MemoryLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.counts, key)
}

func (l *MemoryLimiter) Allow(key string, limit int, window time.Duration, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Bersihkan kunci kedaluwarsa agar memori tidak tumbuh tanpa batas.
	if now.Sub(l.lastSweep) > time.Minute {
		for k, v := range l.counts {
			if !now.Before(v.resetAt) {
				delete(l.counts, k)
			}
		}
		l.lastSweep = now
	}

	entry, ok := l.counts[key]
	if !ok || !now.Before(entry.resetAt) {
		entry = windowCount{resetAt: now.Add(window)}
	}
	if entry.count >= limit {
		l.counts[key] = entry
		return false, entry.resetAt.Sub(now)
	}
	entry.count++
	l.counts[key] = entry
	return true, 0
}
