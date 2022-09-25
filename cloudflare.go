package traefik_cloudflare_plugin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

const (
	minRefresh     = 5 * time.Minute
	defaultRefresh = "24h"
)

type Config struct {
	TrustedCIDRs          []string `json:"trustedCIDRs,omitempty"`
	RefreshInterval       string   `json:"refreshInterval,omitempty"`
	OverwriteForwardedFor bool     `json:"overwriteForwardedFor,omitempty"`
}

func CreateConfig() *Config {
	return &Config{
		TrustedCIDRs:          nil,
		RefreshInterval:       defaultRefresh,
		OverwriteForwardedFor: true,
	}
}

type Cloudflare struct {
	next                  http.Handler
	name                  string
	checker               ipChecker
	overwriteForwardedFor bool
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config == nil {
		return nil, errors.New("invalid config")
	}

	c := &Cloudflare{
		next:                  next,
		name:                  name,
		overwriteForwardedFor: config.OverwriteForwardedFor,
	}

	if len(config.TrustedCIDRs) > 0 {
		cidrs := make([]*net.IPNet, 0, len(config.TrustedCIDRs))

		for _, c := range config.TrustedCIDRs {
			_, cidr, err := net.ParseCIDR(c)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR: %w", err)
			}

			cidrs = append(cidrs, cidr)
		}

		c.checker = &staticIPChecker{
			Cidrs: cidrs,
		}
	} else {
		ri, err := time.ParseDuration(config.RefreshInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid refresh interval: %w", err)
		}

		switch {
		case ri <= 0:
			ri = 0
		case ri < minRefresh:
			ri = minRefresh
		}

		checker := &cloudflareIPChecker{
			RefreshInterval: ri,
		}

		err = checker.Refresh(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh Cloudflare IPs: %w", err)
		}

		c.checker = checker
	}

	return c, nil
}

func (c *Cloudflare) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		code := http.StatusBadRequest
		http.Error(w, http.StatusText(code), code)
		return
	}

	sip := net.ParseIP(ip)
	if sip == nil {
		code := http.StatusBadRequest
		http.Error(w, http.StatusText(code), code)
		return
	}

	allow, err := c.checker.CheckIP(r.Context(), sip)
	if err != nil {
		log.Println(err)

		code := http.StatusInternalServerError
		http.Error(w, http.StatusText(code), code)
		return
	}

	if !allow {
		code := http.StatusForbidden
		http.Error(w, http.StatusText(code), code)
		return
	}

	if c.overwriteForwardedFor {
		err = overwriteForwardedFor(r)
		if err != nil {
			code := http.StatusBadRequest
			http.Error(w, http.StatusText(code), code)
			return
		}
	}

	c.next.ServeHTTP(w, r)
}

func overwriteForwardedFor(r *http.Request) error {
	ip := r.Header.Get("CF-Connecting-IP")
	if ip == "" {
		return errors.New("missing CF-Connecting-IP header")
	}

	r.Header.Set("X-Forwarded-For", ip)
	return nil
}
