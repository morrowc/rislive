package trie

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
			desc:    "Empty string in roots is skipped",
			roots:   []string{"", "10.0.0.0/8"},
			wantLen: 1,
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

func TestInsertCIDRWithData(t *testing.T) {
	tree, _ := New()

	err := tree.InsertCIDRWithData("192.168.1.0/24", "gateway-1")
	if err != nil {
		t.Fatalf("InsertCIDRWithData failed: %v", err)
	}

	netw, val, err := tree.LpmWithData(net.ParseIP("192.168.1.50"))
	if err != nil {
		t.Fatalf("LpmWithData failed: %v", err)
	}
	if netw.String() != "192.168.1.0/24" || val != "gateway-1" {
		t.Errorf("unexpected match: %v, %v", netw, val)
	}

	// Invalid CIDR
	if err := tree.InsertCIDRWithData("invalid-cidr", "data"); err == nil {
		t.Errorf("expected error for invalid CIDR in InsertCIDRWithData")
	}

	// Insert duplicate CIDR updates data and returns no error
	if err := tree.InsertCIDRWithData("192.168.1.0/24", "gateway-updated"); err != nil {
		t.Errorf("InsertCIDRWithData on existing prefix failed: %v", err)
	}
	_, val2, _ := tree.LpmWithData(net.ParseIP("192.168.1.50"))
	if val2 != "gateway-updated" {
		t.Errorf("expected data to be updated to 'gateway-updated', got: %v", val2)
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

	// Invalid CIDR string for FindCIDR
	if _, found := tree.FindCIDR("invalid-cidr"); found {
		t.Errorf("expected FindCIDR('invalid-cidr') to return false")
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
	if tree.ContainsCIDR("invalid-cidr") {
		t.Errorf("expected ContainsCIDR false for invalid CIDR")
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

	// DeleteCIDR with invalid string
	if err := tree.DeleteCIDR("not-a-cidr"); err == nil {
		t.Errorf("expected error for invalid CIDR in DeleteCIDR")
	}

	// Delete non-existent prefix on branch
	if tree.Delete(mustCIDR("192.168.1.0/24")) {
		t.Errorf("expected false deleting non-existent prefix")
	}
}

func TestPrefixesAndWalk(t *testing.T) {
	prefixes := []string{
		"10.0.0.0/8",
		"192.168.1.0/24",
		"2001:db8::/32",
		"2001:db8:1::/48",
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

	// Test early termination of Walk during IPv4
	walkCount := 0
	tree.Walk(func(_ *Node) bool {
		walkCount++
		return false // stop immediately
	})
	if walkCount != 1 {
		t.Errorf("expected walk to terminate at 1, got %d", walkCount)
	}

	// Test early termination of Walk during IPv6
	v6Count := 0
	tree.Walk(func(n *Node) bool {
		if n.Prefix != nil && n.Prefix.Network.IP.To4() == nil {
			v6Count++
			return false // stop after first IPv6
		}
		return true
	})
	if v6Count != 1 {
		t.Errorf("expected v6 walk to stop after 1, got %d", v6Count)
	}
}

func TestNodeSearch(t *testing.T) {
	tree, _ := New("192.168.0.0/16", "192.168.1.0/24")

	// Search on node directly
	got, err := tree.v4.Search(net.ParseIP("192.168.1.1"))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if got.String() != "192.168.1.0/24" {
		t.Errorf("got %s, want 192.168.1.0/24", got)
	}

	// Search matching root/top prefix
	gotRoot, err := tree.v4.Search(net.ParseIP("192.168.2.1"))
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if gotRoot.String() != "192.168.0.0/16" {
		t.Errorf("got %s, want 192.168.0.0/16", gotRoot)
	}

	// Search with no matching prefix in subtree
	treeEmpty, _ := New()
	_, err = treeEmpty.v4.Search(net.ParseIP("192.168.1.1"))
	if err == nil {
		t.Errorf("expected error searching on empty node subtree")
	}

	// Search with nil IP
	_, err = tree.v4.Search(nil)
	if err == nil {
		t.Errorf("expected error searching for nil IP")
	}

	// Search with invalid IP length
	_, err = tree.v4.Search(net.IP{1, 2, 3})
	if err == nil {
		t.Errorf("expected error searching with invalid length IP")
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

func TestSplitIPAndParseCanonicalizeEdgeCases(t *testing.T) {
	// splitIP nil IP
	if _, _, _, err := splitIP(nil); err == nil {
		t.Errorf("expected error from splitIP(nil)")
	}

	// splitIP invalid length
	if _, _, _, err := splitIP(net.IP{1, 2, 3}); err == nil {
		t.Errorf("expected error from splitIP with 3 bytes")
	}

	// parseAndCanonicalize nil or invalid net
	if _, _, _, _, _, err := parseAndCanonicalize(nil); err == nil {
		t.Errorf("expected error from parseAndCanonicalize(nil)")
	}
	if _, _, _, _, _, err := parseAndCanonicalize(&net.IPNet{IP: nil, Mask: net.CIDRMask(24, 32)}); err == nil {
		t.Errorf("expected error from parseAndCanonicalize with nil IP")
	}
	if _, _, _, _, _, err := parseAndCanonicalize(&net.IPNet{IP: net.ParseIP("10.0.0.1"), Mask: nil}); err == nil {
		t.Errorf("expected error from parseAndCanonicalize with nil Mask")
	}
	if _, _, _, _, _, err := parseAndCanonicalize(&net.IPNet{IP: net.ParseIP("10.0.0.1"), Mask: net.IPMask{0xff}}); err == nil {
		t.Errorf("expected error from parseAndCanonicalize with invalid mask size")
	}
	// Mismatched IP family and mask size
	if _, _, _, _, _, err := parseAndCanonicalize(&net.IPNet{IP: net.IP{1, 2, 3}, Mask: net.CIDRMask(24, 32)}); err == nil {
		t.Errorf("expected error from parseAndCanonicalize with 3-byte IP")
	}
}

func TestNilTreeReceiverMethods(t *testing.T) {
	var nilTree *Tree

	if nilTree.Insert(mustCIDR("10.0.0.0/8")) {
		t.Errorf("expected false from nilTree.Insert")
	}
	if err := nilTree.InsertCIDR("10.0.0.0/8"); err == nil {
		t.Errorf("expected error from nilTree.InsertCIDR")
	}
	if nilTree.InsertWithData(mustCIDR("10.0.0.0/8"), "val") {
		t.Errorf("expected false from nilTree.InsertWithData")
	}
	if err := nilTree.InsertCIDRWithData("10.0.0.0/8", "val"); err == nil {
		t.Errorf("expected error from nilTree.InsertCIDRWithData")
	}
	if _, err := nilTree.Lpm(net.ParseIP("10.0.0.1")); err == nil {
		t.Errorf("expected error from nilTree.Lpm")
	}
	if _, err := nilTree.PrefixLpm(mustCIDR("10.0.0.0/8")); err == nil {
		t.Errorf("expected error from nilTree.PrefixLpm")
	}
	if _, _, err := nilTree.LpmWithData(net.ParseIP("10.0.0.1")); err == nil {
		t.Errorf("expected error from nilTree.LpmWithData")
	}
	if _, found := nilTree.Find(mustCIDR("10.0.0.0/8")); found {
		t.Errorf("expected false from nilTree.Find")
	}
	if _, found := nilTree.FindCIDR("10.0.0.0/8"); found {
		t.Errorf("expected false from nilTree.FindCIDR")
	}
	if nilTree.Contains(net.ParseIP("10.0.0.1")) {
		t.Errorf("expected false from nilTree.Contains")
	}
	if nilTree.ContainsCIDR("10.0.0.0/8") {
		t.Errorf("expected false from nilTree.ContainsCIDR")
	}
	if _, err := nilTree.CoveringPrefix(mustCIDR("10.0.0.0/8")); err == nil {
		t.Errorf("expected error from nilTree.CoveringPrefix")
	}
	if res := nilTree.CoveringPrefixes(mustCIDR("10.0.0.0/8")); res != nil {
		t.Errorf("expected nil from nilTree.CoveringPrefixes")
	}
	if res := nilTree.Subnets(mustCIDR("10.0.0.0/8")); res != nil {
		t.Errorf("expected nil from nilTree.Subnets")
	}
	if nilTree.Delete(mustCIDR("10.0.0.0/8")) {
		t.Errorf("expected false from nilTree.Delete")
	}
	if err := nilTree.DeleteCIDR("10.0.0.0/8"); err == nil {
		t.Errorf("expected error from nilTree.DeleteCIDR")
	}
	if nilTree.Size() != 0 {
		t.Errorf("expected 0 from nilTree.Size()")
	}
	if nilTree.Prefixes() != nil {
		t.Errorf("expected nil from nilTree.Prefixes()")
	}
	nilTree.Walk(func(_ *Node) bool { return true })
	if nilTree.getV4Root() != nil || nilTree.getV6Root() != nil {
		t.Errorf("expected nil roots from nilTree")
	}
}

func TestUninitializedTreeRoots(t *testing.T) {
	// Directly constructed empty Tree struct without calling New()
	rawTree := &Tree{}

	if rawTree.getV4Root() == nil || rawTree.getV6Root() == nil {
		t.Errorf("expected non-nil roots initialized on demand")
	}

	// Insert on rawTree
	if !rawTree.Insert(mustCIDR("192.168.1.0/24")) {
		t.Errorf("failed to insert into rawTree")
	}
	if rawTree.Size() != 1 {
		t.Errorf("expected size 1, got %d", rawTree.Size())
	}

	// Tree with legacy Root field set
	legacyRoot := &Node{}
	legacyTree := &Tree{Root: legacyRoot}
	if legacyTree.getV4Root() != legacyRoot {
		t.Errorf("expected getV4Root to reuse legacy Root")
	}
}

func TestDeleteRightChildAndIntermediateNodes(t *testing.T) {
	tree, _ := New()

	// 10.1.1.128/25 has bit 24 = 1, which branches to the right child (r)
	_ = tree.InsertCIDR("10.1.1.128/25")
	if tree.Size() != 1 {
		t.Fatalf("expected size 1, got %d", tree.Size())
	}

	// Deleting it should prune the right child and its parent chain
	if !tree.Delete(mustCIDR("10.1.1.128/25")) {
		t.Errorf("failed to delete 10.1.1.128/25")
	}
	if tree.Size() != 0 {
		t.Errorf("expected size 0 after deletion, got %d", tree.Size())
	}

	// Insert parent and child, then delete the parent
	_ = tree.InsertCIDR("10.0.0.0/8")
	_ = tree.InsertCIDR("10.1.0.0/16")
	if tree.Size() != 2 {
		t.Fatalf("expected size 2, got %d", tree.Size())
	}

	// Deleting the /8 parent should remove its prefix but preserve /16 child
	if !tree.Delete(mustCIDR("10.0.0.0/8")) {
		t.Errorf("failed to delete /8 parent")
	}
	if tree.Size() != 1 {
		t.Errorf("expected size 1, got %d", tree.Size())
	}
	if !tree.ContainsCIDR("10.1.0.0/16") {
		t.Errorf("expected child 10.1.0.0/16 to remain in tree")
	}
	if tree.ContainsCIDR("10.0.0.0/8") {
		t.Errorf("expected parent 10.0.0.0/8 to be deleted")
	}
}

func TestUnpopulatedFamilyQueries(t *testing.T) {
	// A Tree struct with only v4 populated (v6 is nil)
	tree := &Tree{
		v4: &Node{},
	}

	// Querying IPv6 on unpopulated v6 root
	v6Net := mustCIDR("2001:db8::/32")
	if _, err := tree.CoveringPrefix(v6Net); err == nil {
		t.Errorf("expected error from CoveringPrefix for unpopulated v6")
	}
	if res := tree.CoveringPrefixes(v6Net); res != nil {
		t.Errorf("expected nil from CoveringPrefixes for unpopulated v6")
	}
	if res := tree.Subnets(v6Net); res != nil {
		t.Errorf("expected nil from Subnets for unpopulated v6")
	}
	if tree.Delete(v6Net) {
		t.Errorf("expected false from Delete for unpopulated v6")
	}
	if _, found := tree.Find(v6Net); found {
		t.Errorf("expected false from Find for unpopulated v6")
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
			_ = tree.InsertCIDR(cidr)
		}(i)
	}

	// Concurrently query LPM
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(octet int) {
			defer wg.Done()
			ip := net.ParseIP(fmt.Sprintf("10.%d.1.1", octet))
			_, _ = tree.Lpm(ip)
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
		_ = tree.InsertCIDR(fmt.Sprintf("10.%d.0.0/16", i))
		_ = tree.InsertCIDR(fmt.Sprintf("10.%d.1.0/24", i))
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
