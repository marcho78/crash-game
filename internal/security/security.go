package security

import (
	"crypto/subtle"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type IPRateLimiter struct {
	ips map[string]*rate.Limiter
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*rate.Limiter),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}
}

func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.ips[ip]
	if !exists {
		limiter = rate.NewLimiter(i.r, i.b)
		i.ips[ip] = limiter
	}

	return limiter
}

// CSRF Token management
type CSRFManager struct {
	tokens map[string]time.Time
	mu     sync.RWMutex
}

func NewCSRFManager() *CSRFManager {
	manager := &CSRFManager{
		tokens: make(map[string]time.Time),
	}
	go manager.cleanup()
	return manager
}

func (m *CSRFManager) cleanup() {
	for {
		time.Sleep(time.Hour)
		m.mu.Lock()
		for token, created := range m.tokens {
			if time.Since(created) > 24*time.Hour {
				delete(m.tokens, token)
			}
		}
		m.mu.Unlock()
	}
}

func (m *CSRFManager) ValidateToken(token string, expectedToken string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.tokens[token]; !exists {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) == 1
}
