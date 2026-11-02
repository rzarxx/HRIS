package auth

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiterWindow(t *testing.T) {
	l := NewMemoryLimiter()
	now := time.Unix(1000, 0)
	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow("a", 5, time.Minute, now); !ok {
			t.Fatalf("permintaan ke-%d ditolak", i+1)
		}
	}
	ok, wait := l.Allow("a", 5, time.Minute, now.Add(10*time.Second))
	if ok {
		t.Fatal("permintaan ke-6 harus ditolak")
	}
	if wait != 50*time.Second {
		t.Errorf("waktu tunggu = %s", wait)
	}
	if ok, _ := l.Allow("b", 5, time.Minute, now); !ok {
		t.Error("kunci lain tidak boleh terpengaruh")
	}
	if ok, _ := l.Allow("a", 5, time.Minute, now.Add(time.Minute)); !ok {
		t.Error("setelah jendela berakhir harus lolos lagi")
	}
}

func TestLimiterReset(t *testing.T) {
	l := NewMemoryLimiter()
	now := time.Unix(1000, 0)
	l.Allow("a", 1, time.Minute, now)
	if ok, _ := l.Allow("a", 1, time.Minute, now); ok {
		t.Fatal("harus tertahan")
	}
	l.Reset("a")
	if ok, _ := l.Allow("a", 1, time.Minute, now); !ok {
		t.Error("setelah Reset harus lolos")
	}
}

func TestLimiterSweepsExpiredKeys(t *testing.T) {
	l := NewMemoryLimiter()
	now := time.Unix(1000, 0)
	for i := 0; i < 100; i++ {
		l.Allow(fmt.Sprint("k", i), 5, time.Second, now)
	}
	l.Allow("baru", 5, time.Second, now.Add(2*time.Minute))
	if len(l.counts) != 1 {
		t.Errorf("kunci tersisa = %d", len(l.counts))
	}
}

func TestLimiterConcurrent(t *testing.T) {
	l := NewMemoryLimiter()
	now := time.Unix(1000, 0)
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.Allow("a", 10, time.Minute, now); ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != 10 {
		t.Errorf("lolos = %d, ingin 10", allowed.Load())
	}
}
