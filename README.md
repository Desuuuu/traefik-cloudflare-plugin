# traefik-cloudflare-plugin

[![Tag Badge]][Tag] [![Go Version Badge]][Go Version] [![Build Badge]][Build] [![Go Report Card Badge]][Go Report Card]

Traefik plugin to handle traffic coming from Cloudflare.

## Features

* Only allow traffic originating from Cloudflare
* Rewrite requests `X-Forwarded-For` header with the user IP

## Configuration

### Plugin options

| Key                     | Type            | Default | Description                                                                                                                                     |
|:-----------------------:|:---------------:|:-------:|:-----------------------------------------------------------------------------------------------------------------------------------------------:|
| `trustedCIDRs`          | `[]string`      | `[]`    | Requests coming from a source not matching any of these CIDRs will be terminated with a 403. If empty, it is populated with Cloudflare's CIDRs. |
| `refreshInterval`       | `time.Duration` | `24h`   | When `trustedCIDRs` is empty, Cloudflare's CIDRs will be refreshed after this duration. Using a value of 0 seconds disables the refresh.        |
| `overwriteForwardedFor` | `bool`          | `true`  | When `true`, the request's `X-Forwarded-For` header is replaced by the content of the `CF-Connecting-IP` header.                                |

### Traefik static configuration

```yaml
experimental:
  plugins:
    cloudflare:
      moduleName: github.com/Desuuuu/traefik-cloudflare-plugin
      version: v1.0.0
```

### Dynamic configuration

```yaml
http:
  middlewares:
    cloudflare:
      plugin:
        cloudflare:
          trustedCIDRs: []
          overwriteForwardedFor: true

  routers:
    foo-router:
      rule: Path(`/foo`)
      service: foo-service
      entryPoints:
        - web
      middlewares:
        - cloudflare
```

[Tag]: https://github.com/Desuuuu/traefik-cloudflare-plugin/tags
[Tag Badge]: https://img.shields.io/github/v/tag/Desuuuu/traefik-cloudflare-plugin?sort=semver
[Go Version]: /go.mod
[Go Version Badge]: https://img.shields.io/github/go-mod/go-version/Desuuuu/traefik-cloudflare-plugin
[Build]: https://github.com/Desuuuu/traefik-cloudflare-plugin/actions/workflows/test.yml
[Build Badge]: https://img.shields.io/github/workflow/status/Desuuuu/traefik-cloudflare-plugin/Test
[Go Report Card]: https://goreportcard.com/report/github.com/Desuuuu/traefik-cloudflare-plugin
[Go Report Card Badge]: https://goreportcard.com/badge/github.com/Desuuuu/traefik-cloudflare-plugin
