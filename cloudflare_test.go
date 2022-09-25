package traefik_cloudflare_plugin_test

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cloudflare "github.com/Desuuuu/traefik-cloudflare-plugin"
	"github.com/Desuuuu/traefik-cloudflare-plugin/internal"
	"github.com/stretchr/testify/require"
)

func TestCloudflare(t *testing.T) {
	log.SetOutput(io.Discard)

	t.Run("automatic CIDRs", func(t *testing.T) {
		dc := http.DefaultClient
		defer func() {
			http.DefaultClient = dc
		}()

		http.DefaultClient = &http.Client{
			Transport: &staticJsonTransport{
				Response: `{"result":{"ipv4_cidrs":["172.16.0.0/12"],"ipv6_cidrs":["2001:db8:2::/47"],"etag":"ffffffffffffffffffffffffffffffff"},"success":true,"errors":[],"messages":[]}`,
			},
		}

		cfg := cloudflare.CreateConfig()
		cfg.TrustedCIDRs = nil
		cfg.RefreshInterval = "0s"
		cfg.OverwriteForwardedFor = false

		ctx := context.Background()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		handler, err := cloudflare.New(ctx, next, cfg, "cloudflare")
		require.NoError(t, err)

		t.Run("allowed ipv4", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
		})

		t.Run("disallowed ipv4", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.15.1.1:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusForbidden, res.StatusCode)
		})

		t.Run("allowed ipv6", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "[2001:db8:2:2::1]:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
		})

		t.Run("disallowed ipv6", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "[2001:db8:1:2::1]:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusForbidden, res.StatusCode)
		})
	})

	t.Run("automatic CIDRs periodic update", func(t *testing.T) {
		dc := http.DefaultClient
		defer func() {
			http.DefaultClient = dc
		}()

		now := ptime(time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC))

		internal.Now = func() time.Time {
			return *now
		}

		http.DefaultClient = &http.Client{
			Transport: &staticJsonTransport{
				Response: `{"result":{"ipv4_cidrs":["172.16.0.0/12"],"ipv6_cidrs":["2001:db8:2::/47"],"etag":"ffffffffffffffffffffffffffffffff"},"success":true,"errors":[],"messages":[]}`,
			},
		}

		cfg := cloudflare.CreateConfig()
		cfg.TrustedCIDRs = nil
		cfg.RefreshInterval = "5m"
		cfg.OverwriteForwardedFor = false

		ctx := context.Background()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		handler, err := cloudflare.New(ctx, next, cfg, "cloudflare")
		require.NoError(t, err)

		http.DefaultClient = &http.Client{
			Transport: &staticJsonTransport{
				Response: `{"result":null,"success":false,"errors":[{"code":1000,"message":"ERR"}],"messages":[]}`,
			},
		}

		t.Run("initially up-to-date", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
		})

		t.Run("expired with error", func(t *testing.T) {
			now = ptime(now.Add(time.Hour))

			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusInternalServerError, res.StatusCode)
		})

		t.Run("up-to-date after error", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
		})

		t.Run("expired", func(t *testing.T) {
			now = ptime(now.Add(time.Hour))

			http.DefaultClient = &http.Client{
				Transport: &staticJsonTransport{
					Response: `{"result":{"ipv4_cidrs":["172.16.0.0/24"],"ipv6_cidrs":["2001:db8:2::/47"],"etag":"ffffffffffffffffffffffffffffffff"},"success":true,"errors":[],"messages":[]}`,
				},
			}

			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusForbidden, res.StatusCode)
		})
	})

	t.Run("static CIDRs", func(t *testing.T) {
		cfg := cloudflare.CreateConfig()
		cfg.TrustedCIDRs = []string{"172.16.0.0/12", "2001:db8:2::/47"}
		cfg.OverwriteForwardedFor = false

		ctx := context.Background()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		handler, err := cloudflare.New(ctx, next, cfg, "cloudflare")
		require.NoError(t, err)

		t.Run("allowed ipv4", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
		})

		t.Run("disallowed ipv4", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.15.1.1:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusForbidden, res.StatusCode)
		})

		t.Run("allowed ipv6", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "[2001:db8:2:2::1]:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
		})

		t.Run("disallowed ipv6", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "[2001:db8:1:2::1]:42",
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusForbidden, res.StatusCode)
		})
	})

	t.Run("overwrite X-Forwarded-For", func(t *testing.T) {
		cfg := cloudflare.CreateConfig()
		cfg.TrustedCIDRs = []string{"0.0.0.0/0"}
		cfg.OverwriteForwardedFor = true

		ctx := context.Background()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		handler, err := cloudflare.New(ctx, next, cfg, "cloudflare")
		require.NoError(t, err)

		t.Run("valid", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
				Header: makeHeaders(map[string]string{
					"CF-Connecting-IP": "1.2.3.4",
				}),
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			ff := strings.Join(req.Header.Values("X-Forwarded-For"), ",")
			require.Equal(t, "1.2.3.4", ff)

			res := rr.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
		})

		t.Run("overwrite", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
				Header: makeHeaders(map[string]string{
					"CF-Connecting-IP": "1.2.3.4",
					"X-Forwarded-For":  "2.2.2.2, 3.3.3.3",
				}),
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			ff := strings.Join(req.Header.Values("X-Forwarded-For"), ",")
			require.Equal(t, "1.2.3.4", ff)

			res := rr.Result()
			require.Equal(t, http.StatusOK, res.StatusCode)
		})

		t.Run("missing header", func(t *testing.T) {
			req := &http.Request{
				RemoteAddr: "172.16.1.1:42",
				Header: makeHeaders(map[string]string{
					"X-Forwarded-For": "2.2.2.2, 3.3.3.3",
				}),
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			res := rr.Result()
			require.Equal(t, http.StatusBadRequest, res.StatusCode)
		})
	})
}

func ptime(t time.Time) *time.Time {
	return &t
}

func makeHeaders(m map[string]string) http.Header {
	res := make(http.Header, len(m))

	for k, v := range m {
		res[http.CanonicalHeaderKey(k)] = []string{v}
	}

	return res
}

type staticJsonTransport struct {
	Response string
}

func (t *staticJsonTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(t.Response)),
		Request:    r,
	}, nil
}
