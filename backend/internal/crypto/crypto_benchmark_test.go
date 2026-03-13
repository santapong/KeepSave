package crypto

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestHighLoadEncryptionConcurrent(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	concurrency := 100
	opsPerGoroutine := 100

	var successCount int64
	var errorCount int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				plaintext := []byte(fmt.Sprintf("secret-value-%d-%d", id, j))

				ciphertext, nonce, err := Encrypt(key, plaintext)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				decrypted, err := Decrypt(key, ciphertext, nonce)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				if string(decrypted) != string(plaintext) {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	success := atomic.LoadInt64(&successCount)
	errors := atomic.LoadInt64(&errorCount)

	t.Logf("Concurrent Encryption Test:")
	t.Logf("  Goroutines: %d, Ops/goroutine: %d", concurrency, opsPerGoroutine)
	t.Logf("  Successful: %d, Errors: %d", success, errors)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Encrypt+Decrypt ops/sec: %.0f", float64(success)/elapsed.Seconds())

	if errors > 0 {
		t.Errorf("expected 0 errors, got %d", errors)
	}

	expectedTotal := int64(concurrency * opsPerGoroutine)
	if success != expectedTotal {
		t.Errorf("expected %d successes, got %d", expectedTotal, success)
	}
}

func TestHighLoadEnvelopeEncryptionConcurrent(t *testing.T) {
	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i + 42)
	}

	svc, err := NewService(masterKey)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	concurrency := 50
	opsPerGoroutine := 50

	var successCount int64
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each goroutine generates its own DEK
			dek, err := svc.GenerateDEK()
			if err != nil {
				t.Errorf("goroutine %d: GenerateDEK failed: %v", id, err)
				return
			}

			encDEK, dekNonce, err := svc.EncryptDEK(dek)
			if err != nil {
				t.Errorf("goroutine %d: EncryptDEK failed: %v", id, err)
				return
			}

			decDEK, err := svc.DecryptDEK(encDEK, dekNonce)
			if err != nil {
				t.Errorf("goroutine %d: DecryptDEK failed: %v", id, err)
				return
			}

			for j := 0; j < opsPerGoroutine; j++ {
				plaintext := []byte(fmt.Sprintf("env-var-value-%d-%d-with-some-realistic-length-content", id, j))

				ct, nonce, err := Encrypt(decDEK, plaintext)
				if err != nil {
					t.Errorf("goroutine %d, op %d: Encrypt failed: %v", id, j, err)
					return
				}

				result, err := Decrypt(decDEK, ct, nonce)
				if err != nil {
					t.Errorf("goroutine %d, op %d: Decrypt failed: %v", id, j, err)
					return
				}

				if string(result) != string(plaintext) {
					t.Errorf("goroutine %d, op %d: value mismatch", id, j)
					return
				}

				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	success := atomic.LoadInt64(&successCount)
	t.Logf("Envelope Encryption Load Test:")
	t.Logf("  Goroutines: %d, Ops/goroutine: %d", concurrency, opsPerGoroutine)
	t.Logf("  Successful: %d", success)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Ops/sec: %.0f", float64(success)/elapsed.Seconds())

	expectedTotal := int64(concurrency * opsPerGoroutine)
	if success != expectedTotal {
		t.Errorf("expected %d, got %d", expectedTotal, success)
	}
}

func TestHighLoadLargePayloadEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 7)
	}

	// Test with increasingly large payloads
	sizes := []int{64, 256, 1024, 4096, 16384, 65536}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			plaintext := make([]byte, size)
			for i := range plaintext {
				plaintext[i] = byte(i % 256)
			}

			iterations := 100
			start := time.Now()

			for i := 0; i < iterations; i++ {
				ct, nonce, err := Encrypt(key, plaintext)
				if err != nil {
					t.Fatalf("Encrypt failed at iteration %d: %v", i, err)
				}
				_, err = Decrypt(key, ct, nonce)
				if err != nil {
					t.Fatalf("Decrypt failed at iteration %d: %v", i, err)
				}
			}

			elapsed := time.Since(start)
			throughputMB := float64(size*iterations*2) / elapsed.Seconds() / 1024 / 1024

			t.Logf("  Payload: %d bytes, Iterations: %d, Duration: %v, Throughput: %.1f MB/s",
				size, iterations, elapsed, throughputMB)
		})
	}
}

func BenchmarkEncrypt(b *testing.B) {
	key := make([]byte, 32)
	plaintext := []byte("typical-environment-variable-value-that-might-be-stored")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Encrypt(key, plaintext)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	key := make([]byte, 32)
	plaintext := []byte("typical-environment-variable-value-that-might-be-stored")
	ciphertext, nonce, _ := Encrypt(key, plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Decrypt(key, ciphertext, nonce)
	}
}

func BenchmarkEncryptDecryptRoundTrip(b *testing.B) {
	key := make([]byte, 32)
	plaintext := []byte("typical-environment-variable-value-that-might-be-stored")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ct, nonce, _ := Encrypt(key, plaintext)
		Decrypt(key, ct, nonce)
	}
}

func BenchmarkEncryptParallel(b *testing.B) {
	key := make([]byte, 32)
	plaintext := []byte("typical-environment-variable-value-that-might-be-stored")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			Encrypt(key, plaintext)
		}
	})
}

func BenchmarkDEKGeneration(b *testing.B) {
	masterKey := make([]byte, 32)
	svc, _ := NewService(masterKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GenerateDEK()
	}
}
