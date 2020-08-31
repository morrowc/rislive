package main

import (
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSearch(t *testing.T) {
	ip1 := net.ParseIP("192.168.0.1")
	ip2 := net.ParseIP("192.168.1.1")
	tests := []struct {
		desc    string
		ip      net.IP
		trie    *Tree
		want    *net.IPNet
		wantErr bool
	}{{
		desc: "Failure not an IP",
		trie: &Tree{
			Root: &Node{
				Name: "Node 1",
				Prefix: &Prefix{
					IP: ip1,
				},
			},
		},
		ip:      nil,
		wantErr: true,
	}}

	for _, test := range tests {
		got, err := test.trie.Root.Search(test.ip)
		switch {
		case err != nil && !test.wantErr:
			t.Errorf("[%v]: got error when not expecting one: %v", test.desc, err)
		case err == nil && test.wantErr:
			t.Errorf("[%v]: did not get error when expecting one", test.desc)
		case err == nil:
			if diff := cmp.Diff(got, test.want); diff != "" {
				t.Errorf("[$v]: Diff in got/want(+/-):\n%v\n", test.desc, diff)
			}
		}
	}
}
