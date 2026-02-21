package pool

import (
	"testing"
)

func BenchmarkRequestPoolGet(b *testing.B) {
	pool := NewRequestPool()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := pool.Get("GET", "http://example.com/api/test")
		req.Return()
	}
}

func BenchmarkRequestPoolWithoutPooling(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate creating new request without pooling
		_ = make(map[string]interface{}, 10)
	}
}

func BenchmarkInstrumentedPool(b *testing.B) {
	pool := NewInstrumentedPool()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := pool.Get("POST", "http://example.com/api/benchmark")
		req.SetHeader("Content-Type", "application/json")
		req.Return()
	}
}
