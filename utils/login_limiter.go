package utils

import (
	"strings"
	"sync"
	"time"
)

type loginAttempt struct {
	count       int
	windowStart time.Time
	lockedUntil time.Time
}

type LoginLimiter struct {
	mu     sync.Mutex
	items  map[string]*loginAttempt
	max    int
	window time.Duration
	lock   time.Duration
}

func NewLoginLimiter(max int, window, lock time.Duration) *LoginLimiter {
	l := &LoginLimiter{
		items:  map[string]*loginAttempt{},
		max:    max,
		window: window,
		lock:   lock,
	}
	go l.cleanupLoop()
	return l
}

func (l *LoginLimiter) Blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.items[key]
	if !ok {
		return false, 0
	}
	now := time.Now()
	if now.Before(a.lockedUntil) {
		return true, a.lockedUntil.Sub(now)
	}
	return false, 0
}

func (l *LoginLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	a, ok := l.items[key]
	if ok && now.Before(a.lockedUntil) {
		return
	}
	if !ok || now.Sub(a.windowStart) > l.window {
		a = &loginAttempt{windowStart: now}
		l.items[key] = a
	}

	a.count++
	if a.count >= l.max {
		a.lockedUntil = now.Add(l.lock)
		a.count = 0
		a.windowStart = now
	}
}

func (l *LoginLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.items, key)
}

func (l *LoginLimiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, a := range l.items {
			if now.After(a.lockedUntil) && now.Sub(a.windowStart) > l.window {
				delete(l.items, k)
			}
		}
		l.mu.Unlock()
	}
}

func (l *LoginLimiter) ResetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k := range l.items {
		if strings.HasPrefix(k, prefix) {
			delete(l.items, k)
		}
	}
}

var (
	LoginAttempts = NewLoginLimiter(5, 15*time.Minute, 15*time.Minute)

	IPAttempts = NewLoginLimiter(30, 15*time.Minute, 15*time.Minute)
)
