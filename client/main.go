// Package main implements a rislive server/client for demonstration purposes.
package main

import (
	"flag"

	"github.com/morrowc/rislive"
)

var (
	risFile        = flag.String("risFile", "", "A file of json content, to help in testing.")
	risLive        = flag.String("rislive", "https://ris-live.ripe.net/v1/stream/?format=json", "RIS Live firehose url")
	risClient      = flag.String("risclient", "golang-rislive-morrowc", "Clientname to send to rislive")
	risBufferDepth = flag.Int("buffer", 1000, "Max depth of Ris messages to queue.")
)

func main() {
	rf := rislive.RisFilter{Prefix: []string{"199.168.88.0/22"}}
	r := rislive.NewRisLive(*risLive, *risFile, risclient, rf, buffer)

	r.Listen()
	_ = r.Get(r.Filter)
}
