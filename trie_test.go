package main

import (
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func mustCIDR(cidr string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(fmt.Sprintf("invalid CIDR in test: %s", cidr))
	}
	return ipnet
}

func TestNew(t *testing.T) {
	tests := []struct {
		desc    string
		roots   []string
		wantLen int
		wantErr bool
	}{
		{
			desc:    "Empty tree initialization",
			roots:   nil,
			wantLen: 0,
		},
		{
			desc:    "Single IPv4 root",
			roots:   []string{"192.168.0.0/16"},
			wantLen: 1,
		},
		{
			desc:    "Dual IPv4 and IPv6 roots",
			roots:   []string{"10.0.0.0/8", "2001:db8::/32"},
			wantLen: 2,
		},
		{
			desc:    "Invalid CIDR root",
			roots:   []string{"192.168.0.0/99"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tree, err := New(tc.roots...)
		if tc.wantErr {
			if err == nil {
				t.Errorf("[%s]: expected error, got nil", tc.desc)
			}
			continue
		}
		if err != nil {
			t.Errorf("[%s]: unexpected error: %v", tc.desc, err)
			continue
		}
		if tree.Size() != tc.wantLen {
			t.Errorf("[%s]: expected size %d, got %d", tc.desc, tc.wantLen, tree.Size())
		}
	}
}

func TestIPv4Lpm(t *testing.T) {
	tree, err := New()
	if err != nil {
		t.Fatalf("failed to create tree: %v", err)
	}

	prefixes := []string{
		"0.0.0.0/0",
		"10.0.0.0/8",
		"10.1.0.0/16",
		"10.1.1.0/24",
		"10.1.1.128/25",
		"10.1.1.1/32",
		"192.168.0.0/16",
		"192.168.1.0/24",
	}

	for _, p := range prefixes {
		if err := tree.InsertCIDR(p); err != nil {
			t.Fatalf("failed to insert %s: %v", p, err)
		}
	}

	if tree.Size() != len(prefixes) {
		t.Errorf("expected size %d, got %d", len(prefixes), tree.Size())
	}

	tests := []struct {
		desc    string
		ip      string
		want    string
		wantErr bool
	}{
		{
			desc: "Exact /32 match",
			ip:   "10.1.1.1",
			want: "10.1.1.1/32",
		},
		{
			desc: "Match /25 inside /24",
			ip:   "10.1.1.130",
			want: "10.1.1.128/25",
		},
		{
			desc: "Match /24",
			ip:   "10.1.1.50",
			want: "10.1.1.0/24",
		},
		{
			desc: "Match /16",
			ip:   "10.1.2.50",
			want: "10.1.0.0/16",
		},
		{
			desc: "Match /8",
			ip:   "10.2.3.4",
			want: "10.0.0.0/8",
		},
		{
			desc: "Match /24 in 192.168",
			ip:   "192.168.1.100",
			want: "192.168.1.0/24",
		},
		{
			desc: "Match /16 in 192.168",
			ip:   "192.168.2.100",
			want: "192.168.0.0/16",
		},
		{
			desc: "Fallback to default route 0.0.0.0/0",
			ip:   "172.16.0.1",
			want: "0.0.0.0/0",
		},
		{
			desc:    "Nil IP lookup",
			ip:      "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		var ip net.IP
		if tc.ip != "" {
			ip = net.ParseIP(tc.ip)
		}
		got, err := tree.Lpm(ip)
		if tc.wantErr {
			if err == nil {
				t.Errorf("[%s]: expected error, got %v", tc.desc, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("[%s]: unexpected error: %v", tc.desc, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("[%s]: got %s, want %s", tc.desc, got.String(), tc.want)
		}
	}
}

func TestIPv6Lpm(t *testing.T) {
	tree, err := New()
	if err != nil {
		t.Fatalf("failed to create tree: %v", err)
	}

	prefixes := []string{
		"::/0",
		"2001:db8::/32",
		"2001:db8:1::/48",
		"2001:db8:1:2::/64",
		"2001:db8:1:2::1/128",
		"fe80::/10",
	}

	for _, p := range prefixes {
		if err := tree.InsertCIDR(p); err != nil {
			t.Fatalf("failed to insert %s: %v", p, err)
		}
	}

	tests := []struct {
		desc string
		ip   string
		want string
	}{
		{
			desc: "Exact /128 match",
			ip:   "2001:db8:1:2::1",
			want: "2001:db8:1:2::1/128",
		},
		{
			desc: "Match /64",
			ip:   "2001:db8:1:2::5",
			want: "2001:db8:1:2::/64",
		},
		{
			desc: "Match /48",
			ip:   "2001:db8:1:3::1",
			want: "2001:db8:1::/48",
		},
		{
			desc: "Match /32",
			ip:   "2001:db8:2::1",
			want: "2001:db8::/32",
		},
		{
			desc: "Match fe80::/10",
			ip:   "fe80::1",
			want: "fe80::/10",
		},
		{
			desc: "Default route ::/0",
			ip:   "2607:f8b0:4005:805::200e",
			want: "::/0",
		},
	}

	for _, tc := range tests {
		ip := net.ParseIP(tc.ip)
		got, err := tree.Lpm(ip)
		if err != nil {
			t.Errorf("[%s]: unexpected error: %v", tc.desc, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("[%s]: got %s, want %s", tc.desc, got.String(), tc.want)
		}
	}
}

func TestNoMatchWithoutDefaultRoute(t *testing.T) {
	tree, err := New("10.0.0.0/8")
	if err != nil {
		t.Fatalf("failed to create tree: %v", err)
	}

	_, err = tree.Lpm(net.ParseIP("192.168.1.1"))
	if err == nil {
		t.Errorf("expected error for unrouted IP, got nil")
	}

	_, err = tree.Lpm(net.ParseIP("2001:db8::1"))
	if err == nil {
		t.Errorf("expected error for unrouted IPv6, got nil")
	}
}

func TestInsertWithDataAndLpmWithData(t *testing.T) {
	tree, err := New()
	if err != nil {
		t.Fatalf("failed to create tree: %v", err)
	}

	type RouteMeta struct {
		NextHop string
		ASPath  []int32
	}

	meta1 := RouteMeta{NextHop: "10.0.0.1", ASPath: []int32{64512, 65001}}
	meta2 := RouteMeta{NextHop: "10.1.0.1", ASPath: []int32{64512, 65002}}

	tree.InsertWithData(mustCIDR("10.0.0.0/8"), meta1)
	tree.InsertWithData(mustCIDR("10.1.0.0/16"), meta2)

	matchedNet, data, err := tree.LpmWithData(net.ParseIP("10.1.2.3"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if matchedNet.String() != "10.1.0.0/16" {
		t.Errorf("got prefix %s, want 10.1.0.0/16", matchedNet)
	}

	gotMeta, ok := data.(RouteMeta)
	if !ok {
		t.Fatalf("data is not RouteMeta: %T", data)
	}
	if diff := cmp.Diff(gotMeta, meta2); diff != "" {
		t.Errorf("metadata diff (-got +want):\n%s", diff)
	}
}

func TestPrefixLpm(t *testing.T) {
	tree, _ := New("10.0.0.0/8", "10.1.0.0/16")

	got, err := tree.PrefixLpm(mustCIDR("10.1.2.0/24"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "10.1.0.0/16" {
		t.Errorf("got %s, want 10.1.0.0/16", got)
	}

	_, err = tree.PrefixLpm(nil)
	if err == nil {
		t.Errorf("expected error for nil IPNet prefix LPM, got nil")
	}
}

func TestCoveringPrefixAndPrefixes(t *testing.T) {
	tree, _ := New(
		"0.0.0.0/0",
		"10.0.0.0/8",
		"10.1.0.0/16",
		"10.1.1.0/24",
	)

	// Single covering prefix (most specific)
	covering, err := tree.CoveringPrefix(mustCIDR("10.1.1.0/28"))
	if err != nil {
		t.Fatalf("CoveringPrefix error: %v", err)
	}
	if covering.String() != "10.1.1.0/24" {
		t.Errorf("CoveringPrefix got %s, want 10.1.1.0/24", covering)
	}

	// Multiple covering prefixes
	allCovering := tree.CoveringPrefixes(mustCIDR("10.1.1.0/28"))
	want := []string{"0.0.0.0/0", "10.0.0.0/8", "10.1.0.0/16", "10.1.1.0/24"}
	if len(allCovering) != len(want) {
		t.Fatalf("got %d covering prefixes, want %d", len(allCovering), len(want))
	}
	for i, c := range allCovering {
		if c.String() != want[i] {
			t.Errorf("at index %d: got %s, want %s", i, c.String(), want[i])
		}
	}

	// No covering prefix for uninserted family/prefix
	_, err = tree.CoveringPrefix(mustCIDR("2001:db8::/32"))
	if err == nil {
		t.Errorf("expected error for IPv6 covering prefix when none inserted, got nil")
	}
}

func TestSubnets(t *testing.T) {
	tree, _ := New(
		"10.0.0.0/8",
		"10.1.0.0/16",
		"10.1.1.0/24",
		"10.1.2.0/24",
		"192.168.1.0/24",
	)

	subnets := tree.Subnets(mustCIDR("10.1.0.0/16"))
	want := map[string]bool{
		"10.1.0.0/16": true,
		"10.1.1.0/24": true,
		"10.1.2.0/24": true,
	}

	if len(subnets) != len(want) {
		t.Fatalf("got %d subnets, want %d", len(subnets), len(want))
	}
	for _, s := range subnets {
		if !want[s.String()] {
			t.Errorf("unexpected subnet in results: %s", s)
		}
	}

	// Subnet query for a prefix with no children
	subnetsEmpty := tree.Subnets(mustCIDR("172.16.0.0/12"))
	if len(subnetsEmpty) != 0 {
		t.Errorf("expected 0 subnets, got %d", len(subnetsEmpty))
	}
}

func TestFindAndContains(t *testing.T) {
	tree, _ := New("192.168.1.0/24", "2001:db8::/32")

	// Exact match tests
	node, found := tree.Find(mustCIDR("192.168.1.0/24"))
	if !found || node == nil {
		t.Errorf("expected to find 192.168.1.0/24")
	}

	node6, found6 := tree.FindCIDR("2001:db8::/32")
	if !found6 || node6 == nil {
		t.Errorf("expected to find 2001:db8::/32")
	}

	_, foundMissing := tree.FindCIDR("192.168.2.0/24")
	if foundMissing {
		t.Errorf("found non-existent CIDR")
	}

	// Contains tests
	if !tree.Contains(net.ParseIP("192.168.1.50")) {
		t.Errorf("expected tree to contain IP 192.168.1.50")
	}
	if tree.Contains(net.ParseIP("10.0.0.1")) {
		t.Errorf("expected tree to not contain 10.0.0.1")
	}

	if !tree.ContainsCIDR("192.168.1.0/24") {
		t.Errorf("expected ContainsCIDR true")
	}
	if tree.ContainsCIDR("10.0.0.0/8") {
		t.Errorf("expected ContainsCIDR false")
	}
}

func TestDeleteAndPruning(t *testing.T) {
	tree, _ := New("10.0.0.0/8", "10.1.1.0/24")

	if tree.Size() != 2 {
		t.Fatalf("expected size 2, got %d", tree.Size())
	}

	// Delete leaf node with branch pruning
	if !tree.Delete(mustCIDR("10.1.1.0/24")) {
		t.Errorf("failed to delete 10.1.1.0/24")
	}
	if tree.Size() != 1 {
		t.Errorf("expected size 1 after deletion, got %d", tree.Size())
	}

	// Lookup for IP previously matching the deleted prefix should now fallback to /8
	got, err := tree.Lpm(net.ParseIP("10.1.1.50"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "10.0.0.0/8" {
		t.Errorf("got %s, want 10.0.0.0/8", got)
	}

	// Deleting already deleted prefix should return false
	if tree.Delete(mustCIDR("10.1.1.0/24")) {
		t.Errorf("expected delete to return false for already deleted prefix")
	}

	// Delete via DeleteCIDR
	if err := tree.DeleteCIDR("10.0.0.0/8"); err != nil {
		t.Errorf("DeleteCIDR failed: %v", err)
	}
	if tree.Size() != 0 {
		t.Errorf("expected size 0, got %d", tree.Size())
	}

	// DeleteCIDR error when not found
	if err := tree.DeleteCIDR("10.0.0.0/8"); err == nil {
		t.Errorf("expected error deleting missing prefix via DeleteCIDR")
	}
}

func TestPrefixesAndWalk(t *testing.T) {
	prefixes := []string{
		"10.0.0.0/8",
		"192.168.1.0/24",
		"2001:db8::/32",
	}

	tree, _ := New(prefixes...)

	all := tree.Prefixes()
	if len(all) != len(prefixes) {
		t.Fatalf("expected %d prefixes, got %d", len(prefixes), len(all))
	}

	var walked []string
	tree.Walk(func(n *Node) bool {
		walked = append(walked, n.Prefix.String())
		return true
	})

	if len(walked) != len(prefixes) {
		t.Errorf("walked %d prefixes, want %d", len(walked), len(prefixes))
	}

	// Test early termination of Walk
	walkCount := 0
	tree.Walk(func(n *Node) bool {
		walkCount++
		return false // stop immediately
	})
	if walkCount != 1 {
		t.Errorf("expected walk to terminate at 1, got %d", walkCount)
	}
}

func TestNodeSearch(t *testing.T) {
	tree, _ := New("192.168.0.0/16")

	// Search on node directly
	got, err := tree.v4.Search(net.ParseIP("192.168.1.1"))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if got.String() != "192.168.0.0/16" {
		t.Errorf("got %s, want 192.168.0.0/16", got)
	}

	// Search with nil IP
	_, err = tree.v4.Search(nil)
	if err == nil {
		t.Errorf("expected error searching for nil IP")
	}

	// Search with nil Node
	var nilNode *Node
	_, err = nilNode.Search(net.ParseIP("192.168.1.1"))
	if err == nil {
		t.Errorf("expected error searching on nil node")
	}
}

func TestNodeAndPrefixAccessors(t *testing.T) {
	p := &Prefix{
		IP:      net.ParseIP("192.168.1.0"),
		Network: mustCIDR("192.168.1.0/24"),
	}

	if p.GetIP().String() != "192.168.1.0" {
		t.Errorf("p.GetIP() mismatch")
	}
	if p.GetNet().String() != "192.168.1.0/24" {
		t.Errorf("p.GetNet() mismatch")
	}
	if p.String() != "192.168.1.0/24" {
		t.Errorf("p.String() mismatch")
	}

	var nilPrefix *Prefix
	if nilPrefix.GetIP() != nil || nilPrefix.GetNet() != nil || nilPrefix.String() != "<nil>" {
		t.Errorf("nilPrefix accessors failed")
	}

	node := &Node{Prefix: p}
	if node.GetIP().String() != "192.168.1.0" {
		t.Errorf("node.GetIP() mismatch")
	}
	if node.GetNet().String() != "192.168.1.0/24" {
		t.Errorf("node.GetNet() mismatch")
	}

	var nilNode *Node
	if nilNode.GetIP() != nil || nilNode.GetNet() != nil {
		t.Errorf("nilNode accessors failed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	tree, _ := New()
	var wg sync.WaitGroup

	// Concurrently insert prefixes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(octet int) {
			defer wg.Done()
			cidr := fmt.Sprintf("10.%d.0.0/16", octet)
			tree.InsertCIDR(cidr)
		}(i)
	}

	// Concurrently query LPM
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(octet int) {
			defer wg.Done()
			ip := net.ParseIP(fmt.Sprintf("10.%d.1.1", octet))
			tree.Lpm(ip)
		}(i)
	}

	wg.Wait()

	if tree.Size() != 50 {
		t.Errorf("expected size 50, got %d", tree.Size())
	}
}

func BenchmarkLpm(b *testing.B) {
	tree, _ := New()
	for i := 0; i < 256; i++ {
		tree.InsertCIDR(fmt.Sprintf("10.%d.0.0/16", i))
		tree.InsertCIDR(fmt.Sprintf("10.%d.1.0/24", i))
	}
	ip := net.ParseIP("10.128.1.55")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tree.Lpm(ip)
	}
}

func BenchmarkInsert(b *testing.B) {
	cidrs := make([]*net.IPNet, 256)
	for i := 0; i < 256; i++ {
		cidrs[i] = mustCIDR(fmt.Sprintf("172.16.%d.0/24", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree, _ := New()
		for _, c := range cidrs {
			tree.Insert(c)
		}
	}
}
