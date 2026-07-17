package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimit allows at most max requests per fixed window per client IP.
// It exists to slow credential stuffing on the auth endpoints, not to be a
// general traffic shaper, so a coarse fixed window is enough. Counters live
// in memory and reset on restart.
func RateLimit(max int, window time.Duration) gin.HandlerFunc {
	type bucket struct {
		count int
		reset time.Time
	}
	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	return func(c *gin.Context) {
		now := time.Now()
		mu.Lock()
		// Bound memory under address-spoofing floods: drop expired buckets
		// once the map grows unusually large.
		if len(buckets) > 10_000 {
			for ip, b := range buckets {
				if now.After(b.reset) {
					delete(buckets, ip)
				}
			}
		}
		ip := c.ClientIP()
		b := buckets[ip]
		if b == nil || now.After(b.reset) {
			b = &bucket{reset: now.Add(window)}
			buckets[ip] = b
		}
		b.count++
		blocked := b.count > max
		mu.Unlock()

		if blocked {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "请求过于频繁，请稍后再试",
			})
			return
		}
		c.Next()
	}
}

// BodyLimit rejects request bodies larger than n bytes. Dashboard payloads
// are small JSON documents; anything bigger is a mistake or abuse.
func BodyLimit(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		}
		c.Next()
	}
}
