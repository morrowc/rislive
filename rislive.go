// Package rislive implements a service to listen to the RIPE RIS Live service,
// Messages from RIS Live are parsed and sent to a channel for use be clients.
// There are filter capabilities for clients:
//  ASPaths - monitor for prefixes matching an as-path fragment (slice)
//  InvalidTransitAS - monitor for prefixes transiting an AS that shouldn't transit that AS. (map)
//  Origins - monitor for prefixes with designated origins (slice)
//  Prefix - monitor for a designated set of prefixes (slice)
//
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"

	log "github.com/golang/glog"
)

var (
	risFile   = flag.String("risFile", "", "A file of json content, to help in testing.")
	risLive   = flag.String("rislive", "https://ris-live.ripe.net/v1/stream/?format=json", "RIS Live firehose url")
	risClient = flag.String("risclient", "golang-rislive-morrowc", "Clientname to send to rislive")
	buffer    = flag.Int("buffer", 1000, "Max depth of Ris messages to queue.")
)

// RisLive is a struct to hold basic data used in connecting to the RIS Live service
// and managing data output/collection for the calling client.
// TODO(morrowc): Why are the struct elements here Exported? unexport please.
type RisLive struct {
	URL     *string
	File    *string
	UA      *string
	Filter  *RisFilter
	Records int64
	Chan    chan RisMessage
}

// RisFilter is an object to hold content used to filter the collected BGP
// routes before display to the caller.
type RisFilter struct {
	ASPath           []int32        // Asath: [701, 7018, 3356] a fragment of the aspath seen.
	InvalidTransitAS map[int32]bool // {"701":true, "3356":true}.
	Origins          []string       // A list of interesting origin ASH.
	Prefix           []string       // Prefix: ["1.2.3.0/24", "2001:db8::/32"] a list of prefixes.
}

// RisMessage is a single ris_message json message from the ris firehose.
type RisMessage struct {
	Type string          `json:"type"`
	Data *RisMessageData `json:"data"`
}

// RisMessageData is the BGP oriented content of the single RisMessage message type.
type RisMessageData struct {
	Timestamp     float64       `json:"timestamp"`
	Peer          string        `json:"peer"`
	PeerASN       string        `json:"peer_asn,omitempty"`
	ID            string        `json:"id"`
	Host          string        `json:"host"`
	Type          string        `json:"type"`
	Path          []interface{} `json:"path"`
	DigestedPath  []int32
	Community     [][]int32          `json:"community"`
	Origin        string             `json:"origin"`
	Announcements []*RisAnnouncement `json:"announcements"`
	Raw           string             `json:"raw"`
}

// MatchASPath matches a fragment of an aspath with an as-path in an announcement.
func (r *RisMessageData) MatchASPath(c []int32) bool {
	cLen := len(c)
	// If the announcement's aspath is shorter than the candidate, no match is possible.
	if len(r.DigestedPath) < cLen {
		return false
	}
	// Slide the candidate along the announcement path checking for a match.
	for i := 0; i+cLen < len(r.DigestedPath); i++ {
		frag := r.DigestedPath[i:(i + cLen)]
		if reflect.DeepEqual(frag, c) {
			return true
		}
	}
	return false
}

// InvalidTransitAS matches a set of ASN in the RisMessageData.Path, returning true if
// there is a match in the Path. This should be used to alert on invalid paths seen, paths
// which do not match intent/expectations of the announcing ASN.
func (r *RisMessageData) InvalidTransitAS(c map[int32]bool) bool {
	for _, p := range r.DigestedPath {
		if c[p] {
			return true
		}
	}
	return false
}

// CheckOrigins checks the message's bgp Origin Attribute matches a list of possible origins.
func (r *RisMessageData) CheckOrigins(origins []string) bool {
	for _, origin := range origins {
		if r.Origin == origin {
			return true
		}
	}
	return false
}

// RisAnnouncement is a struct which holds the prefixes contained in the single Bgp Message.
type RisAnnouncement struct {
	NextHop  string   `json:"next_hop"`
	Prefixes []string `json:"prefixes"`
}

// MatchPrefix matches a list of prefixes against an announcement's included prefixes.
// Is an exact match, does not implement any super/subnet matching conditions.
func (r *RisAnnouncement) MatchPrefix(cs []string) bool {
	for _, c := range cs {
		for _, p := range r.Prefixes {
			if c == p {
				return true
			}
		}
	}
	return false
}

// NewRisFilter creates a new RisFilter struct.
func NewRisFilter(aspath []int32, transits map[int32]bool, origins, prefix []string) *RisFilter {
	return &RisFilter{
		ASPath:           aspath,
		InvalidTransitAS: transits,
		Origins:          origins,
		Prefix:           prefix,
	}
}

// NewRisLive creates a new RisLive struct.
func NewRisLive(url, file, ua *string, rf *RisFilter, buffer *int) *RisLive {
	return &RisLive{
		URL:     url,
		File:    file,
		UA:      ua,
		Filter:  rf,
		Records: 0,
		Chan:    make(chan (RisMessage), *buffer),
	}
}

func digestPath(m *RisMessageData) error {
	m.DigestedPath = []int32{}
	for _, p := range m.Path {
		var o int32
		switch v := p.(type) {
		// exit loop since both of these can be type cast directly
		// I would log this but no one added a logging package!!!!
		// I would also combine these but typecasting is dumb and
		// without this separation the compiler considers v an interface
		// :(
		case int:
			o = int32(v)
		case float64:
			o = int32(v)
		case []interface{}:
			// Convert p to a slice of interface.
			listSlice, ok := p.([]interface{})
			if !ok {
				return fmt.Errorf("failed to cast path element: %v as %v", p, reflect.TypeOf(p))
			}
			for _, e := range listSlice {
				// I would move this down to the outside of the function but that's difficult
				// and probably not efficient, assuming an input of mostly ints or float64's
				m.DigestedPath = append(m.DigestedPath, int32(e.(float64)))
			}
			// not the cleanest but there's no sane way to clean this up otherwise
			continue
		default:
			return fmt.Errorf("failed to decode path element: %v as %v", p, reflect.TypeOf(p))
		}
		m.DigestedPath = append(m.DigestedPath, o)

	}
	return nil
}

// Listen connects to the RisLive service, parses the stream into structs
// and makes the data stream available for analysis through the RisLive.Chan channel.
func (r *RisLive) Listen() {
	var body io.ReadCloser
	// If there's a file provided read/use that, else open the remote
	// socket and consume the firehose.
	switch len(*r.File) == 0 {
	case true:
		log.Infof("Reading from the firehose...")
		client := &http.Client{}
		req, err := http.NewRequest("GET", *r.URL, nil)
		if err != nil {
			log.Fatalf("failed to create new request to ris-live: %v\n", err)
		}
		req.Header.Set("User-Agent", *r.UA)
		resp, err := client.Do(req)
		defer resp.Body.Close()
		body = resp.Body
	default:
		log.Infof("Heres a file read")
		fd, err := ioutil.ReadFile(*r.File)
		if err != nil {
			log.Fatalf("failed to read risFile(%v): %v\n", *r.File, err)
		}
		body = ioutil.NopCloser(bytes.NewReader(fd))
		log.Infof("Finished Reading File")
	}

	dec := json.NewDecoder(body)

	// Remove log file once done.
	f, err := os.Create("/tmp/log")
	if err != nil {
		log.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()
	for {
		var rm RisMessage
		err := dec.Decode(&rm)
		switch {
		case err != nil && err != io.EOF:
			_, err := f.WriteString(fmt.Sprintf("bad json content: %+v\n", rm.Data))
			if err != nil {
				log.Fatalf("failed to write to log: %v", err)
			}
			continue
		case err == io.EOF:
			close(r.Chan)
			return
		}
		err = digestPath(rm.Data)
		if err != nil {
			fmt.Printf("decoding the message data path(%v) failed: %v\n", rm.Data.Path, err)
			log.Infof("decoding the message data path(%v) failed: %v", rm.Data.Path, err)
		}
		r.Records++
		r.Chan <- rm
	}
}

// Get collects messages from the RisLive.Chan channel and filters results prior
// to display or handling downstream.
// TODO(morrowc): Why is Get accepting a Filter? Why not just use the Filter in RisLive?
func (r *RisLive) Get(f *RisFilter) string {
	for rm := range r.Chan {
		rmd := rm.Data
		prefix := ""
		// Pull a single prefix from the announcement, which may have more than one.
		if len(rmd.Announcements) > 0 {
			if len(rmd.Announcements[0].Prefixes) > 0 {
				prefixes := []string{}
				for _, a := range rmd.Announcements {
					for _, p := range a.Prefixes {
						prefixes = append(prefixes, p)
					}
					fmt.Printf("Prefixes: %v Origin: %v Path: %v\n",
						strings.Join(prefixes, ", "),
						rmd.DigestedPath[len(rmd.DigestedPath)-1],
						rmd.Path)
				}
			}
		}
		log.Infof("Got a prefix: %v / announcement\n", prefix)
		// TODO(morrowc): This doesn't appear to be working properly.
		// the logic here needs to be more complicated, depending upon what's set
		// in the filter to check. Suggest make 'checkTests' like function, evaluate
		// so only the set filter parts matter.
		if r.CheckASPath(rmd) && r.CheckInvalidTransitAS(rmd) &&
			r.CheckOrigins(rmd) && r.CheckPrefix(rmd) {
			return fmt.Sprintf("Message(%d): Peer/ASN -> %v/%v Prefix1: %v\n", r.Records, rmd.Peer, rmd.PeerASN, prefix)
		}
	}
	return "Done"
}

// CheckASPath checks the filterable ASPath, if it's set.
// If not set, always return true.
func (r *RisLive) CheckASPath(rm *RisMessageData) bool {
	if len(r.Filter.ASPath) > 0 {
		return rm.MatchASPath(r.Filter.ASPath)
	}
	return true
}

// CheckInvalidTransitAS checks to see if there is a marked invalid ASN in the as-path.
// If there is no map, this check returns false: there is nothing to match, so no match.
func (r *RisLive) CheckInvalidTransitAS(rm *RisMessageData) bool {
	if len(r.Filter.InvalidTransitAS) > 0 {
		return rm.InvalidTransitAS(r.Filter.InvalidTransitAS)
	}
	return false
}

// CheckOrigins checks the inbound message origin against a list of possible origins.
// If there is no list of origins, return false, an origin must be specified in the filter.
func (r *RisLive) CheckOrigins(rm *RisMessageData) bool {
	if len(r.Filter.Origins) > 0 {
		return rm.CheckOrigins(r.Filter.Origins)
	}
	return false
}

// CheckPrefix will check each announcement in a message, and return true
// if there is a prefix in the message that matches the watched prefixes.
// These are exact matches of strings, there is no super/subnet/covering route
// check being performed, ie:
//   192.168.0.0/16 vs 192.168.0.0/16 - match
//   192.168.0.0/16 vs 192.168.0.0/24 - no match
// TODO(morrowc): Provide super/subnet verification of each announced prefix
// to the requestors list of supernets.
func (r *RisLive) CheckPrefix(rm *RisMessageData) bool {
	if len(r.Filter.Prefix) > 0 {
		filterPrefixes := []*net.IPNet{}
		for _, prefix := range r.Filter.Prefix {
			_, subnet, err := net.ParseCIDR(prefix)
			if err != nil {
				log.Infof("failed to convert filter prefix(%v) to IPNet: %v", prefix, err)
				continue
			}
			filterPrefixes = append(filterPrefixes, subnet)
		}
		for _, anns := range rm.Announcements {
			for _, prefix := range anns.Prefixes {
				for _, check := range filterPrefixes {
					announcementIP, _, err := net.ParseCIDR(prefix)
					if err != nil {
						log.Infof("announcement prefix(%v) not parsed as CIDR: %v", prefix, err)
						continue
					}
					if check.Contains(announcementIP) {
						return true
					}
				}
			}
		}
	}
	return false
}

func main() {
	flag.Parse()
	rf := &RisFilter{
		Prefix:  []string{"130.137.85.0/24", "199.168.88.0/22", "8.8.8.0/24", "8.8.4.0/24", "216.239.32.0/19"},
		Origins: []string{"15169", "54054", "396982"},
	}
	r := NewRisLive(risLive, risFile, risClient, rf, buffer)

	go r.Listen()
	result := r.Get(r.Filter)
	fmt.Printf("Result: %v\n", result)
}
