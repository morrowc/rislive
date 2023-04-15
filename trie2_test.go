package trie2

import (
	"net"
	"testing"
)

func TestAF(t *testing.T) {
	tests := []struct {
		desc string
		ip   net.IP
		want int
	}{{
		desc: "192.168.1.1 - v4",
		ip:   net.ParseIP("192.168.1.1"),
		want: 4,
	}, {
		desc: "2001:db8::1 - v6",
		ip:   net.ParseIP("2001:db8::1"),
		want: 6,
	}}

	for _, test := range tests {
		got := af(test.ip)
		if got != test.want {
			t.Errorf("[%v]: got/want mismatch: %d / %d", test.desc, got, test.want)
		}
	}
}

func TestSameFamily(t *testing.T) {
	_, v4192Net, _ := net.ParseCIDR("192.168.1.1/32")
	_, v410Net, _ := net.ParseCIDR("10.1.1.1/32")
	_, v6DbNet, _ := net.ParseCIDR("2001:db8::1/128")
	_, v6GoogNet, _ := net.ParseCIDR("2001:4860:4848::1/128")

	tests := []struct {
		desc string
		a    *net.IPNet
		b    *net.IPNet
		want bool
	}{{
		desc: "Same v4: 192.168.1.1 / 10.1.1.1",
		a:    v4192Net,
		b:    v410Net,
		want: true,
	}, {
		desc: "Same v6: 2001:db8::1 / 2001:4860:4848::1",
		a:    v6DbNet,
		b:    v6GoogNet,
		want: true,
	}, {
		desc: "Different: 192.168.1.1 / 2001:4860:4848::1",
		a:    v4192Net,
		b:    v6GoogNet,
		want: false,
	}, {
		desc: "Different: 2001:4860:4848::1 / 192.168.1.1",
		a:    v6GoogNet,
		b:    v4192Net,
		want: false,
	}}

	for _, test := range tests {
		got := sameFamily(test.a, test.b)
		if got != test.want {
			t.Errorf("[%v]: got/want mismatch: %v / %v", test.desc, got, test.want)
		}
	}
}

/*
func TestLookup(t *testing.T) {
	// Create a new patricia trie.
	trie := NewPatriciaTrie()

	// Insert some IPv4 and IPv6 addresses into the patricia trie.
	v4Net, _, _ := net.ParseCIDR("192.168.1.0/24")
	trie.Insert(v4Net)
	v6Net, _, _ := net.ParseCIDR("2001:db8::/64")
	trie.Insert(v6Net)

	// Lookup an IPv4 address in the patricia trie.
	if !trie.Lookup(net.ParseIP("192.168.1.1")) {
		t.Errorf("Expected 192.168.1.1 to be found in the patricia trie")
	}

	// Lookup an IPv6 address in the patricia trie.
	if !trie.Lookup(net.ParseIP("2001:db8::1")) {
		t.Errorf("Expected 2001:db8::1 to be found in the patricia trie")
	}

	// Lookup an IPv4 address that is not in the patricia trie.
	if trie.Lookup(net.ParseIP("192.168.2.1")) {
		t.Errorf("Expected 192.168.2.1 to not be found in the patricia trie")
	}

	// Lookup an IPv6 address that is not in the patricia trie.
	if trie.Lookup(net.ParseIP("2001:db8::2")) {
		t.Errorf("Expected 2001:db8::2 to not be found in the patricia trie")
	}
}
*/
