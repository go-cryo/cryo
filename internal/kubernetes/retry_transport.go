package kubernetes

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// retryTransport retries apiserver requests that failed at the transport level.
// Without it a single TLS handshake timeout failed a whole backup run or
// dropped a job's schedule. Reads are retried on any transport error; writes
// only when the request provably never left the client (dial or TLS handshake
// failure), so a Create is never sent twice.
type retryTransport struct {
	next     http.RoundTripper
	attempts int
	backoff  time.Duration
}

func newRetryTransport(next http.RoundTripper) http.RoundTripper {
	return &retryTransport{next: next, attempts: 5, backoff: time.Second}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	delay := t.backoff
	for attempt := 1; ; attempt++ {
		resp, err := t.next.RoundTrip(req)
		if err == nil || attempt >= t.attempts || !retryable(req, err) {
			return resp, err
		}

		// The failed attempt consumed the body; a retry needs a fresh one.
		if req.Body != nil && req.Body != http.NoBody {
			if req.GetBody == nil {
				return resp, err
			}
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return resp, err
			}
			req = req.Clone(req.Context())
			req.Body = body
		}

		log.Warn().Err(err).Str("method", req.Method).Str("path", req.URL.Path).Int("attempt", attempt).Msg("apiserver request failed, retrying")
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
		}
		delay *= 2
	}
}

func retryable(req *http.Request, err error) bool {
	if req.Context().Err() != nil {
		return false
	}
	if req.Method == http.MethodGet || req.Method == http.MethodHead {
		return true
	}
	return notSent(err)
}

// notSent reports errors raised before any request bytes reached the server.
func notSent(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return true
	}
	// net/http's tlsHandshakeTimeoutError is unexported; match its message.
	return strings.Contains(err.Error(), "TLS handshake timeout")
}
