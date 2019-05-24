package rislive

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

var (
	risFile   = flag.String("risFile", "", "A file of json content, to help in testing.")
	risLive   = flag.String("rislive", "https://ris-live.ripe.net/v1/stream/?format=json", "RIS Live firehose url")
	risClient = flag.String("risclient", "golang-rislive-morrowc", "Clientname to send to rislive")
)

type RisLive struct{}

// RisMessage is a single ris_message json message from the ris firehose.
type RisMessage struct {
	Type string          `json:"type"`
	Data *RisMessageData `json:"data"`
}

type RisMessageData struct {
	Timestamp     float64           `json:"timestamp"`
	Peer          string            `json:"peer"`
	PeerASN       string            `json:"peer_asn, omitempty"`
	Id            string            `json:"id"`
	Host          string            `json:"host"`
	Type          string            `json:"type"`
	Path          []int32           `json:"path"`
	Community     [][]int32         `json:"community"`
	Origin        string            `json:"origin"`
	Announcements []RisAnnouncement `json:"announcements"`
	Raw           string            `json:"raw"`
}

type RisAnnouncement struct {
	NextHop  string   `json:"next_hop"`
	Prefixes []string `json:"prefixes"`
}

func (r *RisLive) Listen() {
	var body io.ReadCloser
	switch len(*risFile) == 0 {
	case true:
		resp, err := http.Get("https://ris-live.ripe.net/v1/stream/?format=json")
		if err != nil {
			fmt.Printf("failed to connect to ris-live: %v\n", err)
		}
		defer resp.Body.Close()
		body = resp.Body
	case false:
		fd, err := ioutil.ReadFile(*risFile)
		if err != nil {
			fmt.Printf("failed to read risFile(%v): %v\n", *risFile, err)
		}
		body = ioutil.NopCloser(bytes.NewReader(fd))
	}

	dec := json.NewDecoder(body)

	i := 0
	var rm RisMessage
	for dec.More() {
		err := dec.Decode(&rm)
		if err != nil {
			fmt.Printf("failed to decode json: %v\n", err)
			fmt.Printf("bad json content: %s\n", rm)
			return
		}

		m := rm.Data
		prefix := ""
		if len(m.Announcements) > 0 {
			if len(m.Announcements[0].Prefixes) > 0 {
				prefix = m.Announcements[0].Prefixes[0]
			}
		}
		fmt.Printf("Message(%d): Peer/ASN -> %v/%v Prefix1: %v\n", i, m.Peer, m.PeerASN, prefix)
		i++
	}
}
