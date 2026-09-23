package kubernetes

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
)

type flakyTransport struct {
	errs   []error
	calls  int
	bodies []string
}

func (f *flakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.calls++
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		f.bodies = append(f.bodies, string(b))
	}
	if f.calls <= len(f.errs) {
		return nil, f.errs[f.calls-1]
	}
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

var (
	errHandshake = errors.New("net/http: TLS handshake timeout")
	errDial      = &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	errLost      = errors.New("http2: client connection lost")
)

func roundTrip(t *testing.T, flaky *flakyTransport, method string) error {
	t.Helper()
	rt := &retryTransport{next: flaky, attempts: 3}
	req, _ := http.NewRequest(method, "https://apiserver/api/v1/namespaces/ops/configmaps", bytes.NewReader([]byte(`{"x":1}`)))
	_, err := rt.RoundTrip(req)
	return err
}

func TestRetryTransport(t *testing.T) {
	tests := []struct {
		name      string
		method    string
		errs      []error
		wantCalls int
		wantErr   bool
	}{
		{"get retries any transport error", http.MethodGet, []error{errLost, errLost}, 3, false},
		{"post retries tls handshake timeout", http.MethodPost, []error{errHandshake}, 2, false},
		{"post retries dial error", http.MethodPost, []error{errDial}, 2, false},
		{"post does not retry once the request may have been sent", http.MethodPost, []error{errLost}, 1, true},
		{"gives up after max attempts", http.MethodGet, []error{errLost, errLost, errLost, errLost}, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flaky := &flakyTransport{errs: tt.errs}
			err := roundTrip(t, flaky, tt.method)
			if flaky.calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", flaky.calls, tt.wantCalls)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
			for i, b := range flaky.bodies {
				if b != `{"x":1}` {
					t.Errorf("attempt %d sent body %q, want the full body on every retry", i+1, b)
				}
			}
		})
	}
}
