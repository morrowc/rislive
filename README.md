# rislive
![coverage](./coverage_badge.png "Coverage")
![buildstatus](https://api.travis-ci.org/morrowc/rislive.svg?branch=master "BuildStatus")
![](https://github.com/morrowc/rislive/workflows/RisLive/badge.svg)
![goreportcard](https://goreportcard.com/badge/github.com/morrowc/rislive "Go Report Card"
)

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
