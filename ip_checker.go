package traefik_cloudflare_plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Desuuuu/traefik-cloudflare-plugin/internal"
)

type ipChecker interface {
	CheckIP(context.Context, net.IP) (bool, error)
}

type staticIPChecker struct {
	Cidrs []*net.IPNet
}

func (c *staticIPChecker) CheckIP(ctx context.Context, ip net.IP) (bool, error) {
	for _, cidr := range c.Cidrs {
		if cidr.Contains(ip) {
			return true, nil
		}
	}

	return false, nil
}

type cloudflareIPChecker struct {
	RefreshInterval time.Duration

	cidrs       []*net.IPNet
	lastRefresh time.Time
}

func (c *cloudflareIPChecker) CheckIP(ctx context.Context, ip net.IP) (bool, error) {
	if c.RefreshInterval > 0 && internal.Now().Sub(c.lastRefresh) > c.RefreshInterval {
		err := c.Refresh(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to refresh Cloudflare IPs: %w", err)
		}
	}

	for _, cidr := range c.cidrs {
		if cidr.Contains(ip) {
			return true, nil
		}
	}

	return false, nil
}

func (c *cloudflareIPChecker) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.cloudflare.com/client/v4/ips", nil)
	if err != nil {
		c.lastRefresh = internal.Now().Add(5*time.Minute - c.RefreshInterval)
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.lastRefresh = internal.Now().Add(5*time.Minute - c.RefreshInterval)
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		c.lastRefresh = internal.Now().Add(5*time.Minute - c.RefreshInterval)
		return fmt.Errorf("invalid response: %s", res.Status)
	}

	var resp cloudflareResponse

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		c.lastRefresh = internal.Now().Add(5*time.Minute - c.RefreshInterval)
		return err
	}

	cidrs, err := resp.Data()
	if err != nil {
		c.lastRefresh = internal.Now().Add(5*time.Minute - c.RefreshInterval)
		return err
	}

	c.cidrs = cidrs
	c.lastRefresh = internal.Now()
	return nil
}

type cloudflareResponse struct {
	Success bool               `json:"success"`
	Errors  []*cloudflareError `json:"errors"`
	Result  *cloudflareIPs     `json:"result"`
}

func (r *cloudflareResponse) Data() ([]*net.IPNet, error) {
	if !r.Success || r.Result == nil {
		for _, e := range r.Errors {
			err := e.Error()
			if err != nil {
				return nil, err
			}
		}

		return nil, errors.New("invalid response")
	}

	res := make([]*net.IPNet, 0, len(r.Result.IPv4CIDRs)+len(r.Result.IPv6CIDRs))

	for _, c := range r.Result.IPv4CIDRs {
		_, cidr, err := net.ParseCIDR(c)
		if err != nil {
			return nil, err
		}

		res = append(res, cidr)
	}

	for _, c := range r.Result.IPv6CIDRs {
		_, cidr, err := net.ParseCIDR(c)
		if err != nil {
			return nil, err
		}

		res = append(res, cidr)
	}

	return res, nil
}

type cloudflareError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *cloudflareError) Error() error {
	if e == nil {
		return nil
	}

	return fmt.Errorf("Error %d: %s", e.Code, e.Message)
}

type cloudflareIPs struct {
	IPv4CIDRs []string `json:"ipv4_cidrs"`
	IPv6CIDRs []string `json:"ipv6_cidrs"`
}
