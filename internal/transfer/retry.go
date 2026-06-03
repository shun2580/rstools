package transfer

import (
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

// retryableStatus reports whether an HTTP status code should trigger a retry.
func retryableStatus(code int) bool {
	return code == 429 || code == 500 || code == 502 || code == 503
}

// backoff computes the exponential backoff duration with full jitter.
// attempt is 0-indexed. Cap is 30s, base is 1s.
func backoff(attempt int) time.Duration {
	base := time.Second
	cap := 30 * time.Second
	expo := time.Duration(math.Min(float64(cap), float64(base)*math.Pow(2, float64(attempt))))
	jitter := time.Duration(rand.Int63n(int64(expo) + 1))
	return jitter
}

// retryAfter parses the Retry-After header value (seconds).
func retryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	secs, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// WithRetry executes fn up to 4 times (1 attempt + 3 retries),
// retrying on retryable HTTP status codes with exponential backoff.
func WithRetry(fn func() (*http.Response, error)) (*http.Response, error) {
	const maxAttempts = 4
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			wait := backoff(attempt - 1)
			if resp != nil && resp.StatusCode == 429 {
				if ra := retryAfter(resp); ra > wait {
					wait = ra
				}
				resp.Body.Close()
			}
			time.Sleep(wait)
		}

		resp, lastErr = fn()
		if lastErr != nil {
			continue
		}
		if !retryableStatus(resp.StatusCode) {
			return resp, nil
		}
	}
	return resp, lastErr
}
