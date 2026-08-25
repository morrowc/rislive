# rislive

[![RisLive](https://github.com/morrowc/rislive/actions/workflows/rislive.yml/badge.svg)](https://github.com/morrowc/rislive/actions/workflows/rislive.yml)
[![golangci-lint](https://github.com/morrowc/rislive/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/morrowc/rislive/actions/workflows/golangci-lint.yml)
[![CodeQL](https://github.com/morrowc/rislive/workflows/CodeQL/badge.svg)](https://github.com/morrowc/rislive/security/code-scanning)
[![Coverage Status](https://coveralls.io/repos/github/morrowc/rislive/badge.svg?branch=main)](https://coveralls.io/github/morrowc/rislive?branch=main)

(Apache2.0 License Applies)

Golang client to connect to the RIPE RIS Live firehose, and listen for interesting events.

TODO(morrowc):
  * Enable filtering of the view/prefixes properly.
  * Enable RPKI marking based upon CloudFlare's data at:
     https://rpki.cloudflare.com/rpki.json

Coverage and testing:
  * go test -coverprofile=coverage.out
  * go tool cover -func=coverage.out
  * go tool cover -html=coverage.out
