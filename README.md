# rislive

[![RisLive](https://github.com/morrowc/rislive/actions/workflows/rislive.yml/badge.svg)](https://github.com/morrowc/rislive/actions/workflows/rislive.yml)
[![CodeQL](https://github.com/morrowc/rislive/actions/workflows/codeql.yml/badge.svg)](https://github.com/morrowc/rislive/actions/workflows/codeql.yml)
[![Coverage Status](https://coveralls.io/repos/github/morrowc/rislive/badge.svg?branch=main)](https://coveralls.io/github/morrowc/rislive?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/morrowc/rislive)](https://goreportcard.com/report/github.com/morrowc/rislive)

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
