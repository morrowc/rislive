// Package main provides a high-performance, concurrency-safe Patricia Trie (Radix Tree)
// for storing and querying IPv4 and IPv6 subnets / networks.
//
// Supports Longest Prefix Match (LPM), covering prefix (supernet) lookups,
// subnet (child) lookups, exact match, insertion, deletion, and iteration.
//
// TODO(morrowc): this should be moved to an external package (e.g. github.com/morrowc/trie).
package main

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
)

// Tree is the binary Patricia Trie storing IPv4 and IPv6 prefixes.
type Tree struct {
	mu       sync.RWMutex
	Root     *Node // Deprecated/legacy alias pointing to v4 root for backward compatibility.
	v4       *Node // Root node for IPv4 prefix trie.
	v6       *Node // Root node for IPv6 prefix trie.
	elements int32 // Total number of prefixes stored in the tree.
}

// Prefix represents a single node's IP and Network prefix.
type Prefix struct {
	IP      net.IP
	Network *net.IPNet
}

// GetIP returns the prefix's IP address.
func (p *Prefix) GetIP() net.IP {
	if p == nil {
		return nil
	}
	return p.IP
}

// GetNet returns the prefix's net.IPNet network.
func (p *Prefix) GetNet() *net.IPNet {
	if p == nil {
		return nil
	}
	return p.Network
}

// String returns the CIDR string representation of the prefix.
func (p *Prefix) String() string {
	if p == nil || p.Network == nil {
		return "<nil>"
	}
	return p.Network.String()
}

// Node is a single tree element with links to parent and left/right children.
type Node struct {
	Name   string      // An optional identifier, nexthop, or label.
	Prefix *Prefix     // The prefix information for this node, IP and Network.
	Data   interface{} // Optional custom payload/metadata.
	parent *Node       // Parent node.
	l, r   *Node       // Left (bit 0) and Right (bit 1) children.
	lock   *sync.Mutex // Optional mutex for node-level operations.
}

// GetIP returns the Node's IP if a prefix is assigned.
func (n *Node) GetIP() net.IP {
	if n == nil || n.Prefix == nil {
		return nil
	}
	return n.Prefix.GetIP()
}

// GetNet returns the Node's IPNet if a prefix is assigned.
func (n *Node) GetNet() *net.IPNet {
	if n == nil || n.Prefix == nil {
		return nil
	}
	return n.Prefix.GetNet()
}

// getBit returns the bit (0 or 1) at the 0-indexed bit position in bytes.
// Bit 0 is the most significant bit (MSB) of bytes[0].
func getBit(b []byte, bit int) byte {
	byteIdx := bit >> 3
	bitIdx := 7 - (bit & 7)
	return (b[byteIdx] >> bitIdx) & 1
}

// splitIP extracts raw bytes, max bits (32 for IPv4, 128 for IPv6), and family flag.
func splitIP(ip net.IP) ([]byte, int, bool, error) {
	if ip == nil {
		return nil, 0, false, errors.New("cannot process nil IP")
	}
	if ip4 := ip.To4(); ip4 != nil {
		return []byte(ip4), 32, false, nil
	}
	if ip6 := ip.To16(); ip6 != nil {
		return []byte(ip6), 128, true, nil
	}
	return nil, 0, false, fmt.Errorf("invalid IP format: %v", ip)
}

// parseAndCanonicalize canonicalizes an IPNet by masking out host bits and returns normalized values.
func parseAndCanonicalize(n *net.IPNet) (net.IP, *net.IPNet, []byte, int, bool, error) {
	if n == nil || n.IP == nil || n.Mask == nil {
		return nil, nil, nil, 0, false, errors.New("nil or invalid IPNet")
	}
	ones, bits := n.Mask.Size()
	if bits == 0 {
		return nil, nil, nil, 0, false, fmt.Errorf("invalid IPNet mask: %v", n.Mask)
	}

	if ip4 := n.IP.To4(); ip4 != nil && bits == 32 {
		masked := ip4.Mask(n.Mask)
		canonNet := &net.IPNet{
			IP:   masked,
			Mask: net.CIDRMask(ones, 32),
		}
		return masked, canonNet, []byte(masked), ones, false, nil
	}

	if ip6 := n.IP.To16(); ip6 != nil && bits == 128 {
		masked := ip6.Mask(n.Mask)
		canonNet := &net.IPNet{
			IP:   masked,
			Mask: net.CIDRMask(ones, 128),
		}
		return masked, canonNet, []byte(masked), ones, true, nil
	}

	return nil, nil, nil, 0, false, fmt.Errorf("mismatched IP family and mask size: IP=%v, Mask=%v", n.IP, n.Mask)
}

// getV4Root returns the v4 root node, ensuring initialization.
func (t *Tree) getV4Root() *Node {
	if t == nil {
		return nil
	}
	if t.v4 == nil {
		if t.Root != nil {
			t.v4 = t.Root
		} else {
			t.v4 = &Node{}
			t.Root = t.v4
		}
	}
	return t.v4
}

// getV6Root returns the v6 root node, ensuring initialization.
func (t *Tree) getV6Root() *Node {
	if t == nil {
		return nil
	}
	if t.v6 == nil {
		t.v6 = &Node{}
	}
	return t.v6
}

// New creates a new Patricia Trie, optionally pre-populated with root/initial CIDRs.
func New(roots ...string) (*Tree, error) {
	v4 := &Node{}
	v6 := &Node{}
	t := &Tree{
		Root: v4,
		v4:   v4,
		v6:   v6,
	}

	for _, root := range roots {
		if root == "" {
			continue
		}
		if err := t.InsertCIDR(root); err != nil {
			return nil, fmt.Errorf("parsing cidr: %v failed: %w", root, err)
		}
	}
	return t, nil
}

// Insert adds a prefix (as *net.IPNet) to the tree.
// Returns true if the prefix was newly inserted, or false if it was already present.
func (t *Tree) Insert(n *net.IPNet) bool {
	return t.InsertWithData(n, nil)
}

// InsertCIDR parses a CIDR string (e.g. "192.168.1.0/24" or "2001:db8::/32") and inserts it into the tree.
func (t *Tree) InsertCIDR(cidr string) error {
	if t == nil {
		return errors.New("tree is nil")
	}
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("parsing cidr %q: %w", cidr, err)
	}
	t.Insert(ipnet)
	return nil
}

// InsertWithData adds a prefix with an associated data payload.
func (t *Tree) InsertWithData(n *net.IPNet, data interface{}) bool {
	if t == nil {
		return false
	}
	ip, canonNet, rawBytes, prefixLen, isV6, err := parseAndCanonicalize(n)
	if err != nil {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	var root *Node
	if isV6 {
		root = t.getV6Root()
	} else {
		root = t.getV4Root()
	}
	if root == nil {
		return false
	}

	curr := root
	for bit := 0; bit < prefixLen; bit++ {
		b := getBit(rawBytes, bit)
		if b == 0 {
			if curr.l == nil {
				curr.l = &Node{parent: curr}
			}
			curr = curr.l
		} else {
			if curr.r == nil {
				curr.r = &Node{parent: curr}
			}
			curr = curr.r
		}
	}

	isNew := (curr.Prefix == nil)
	curr.Prefix = &Prefix{
		IP:      ip,
		Network: canonNet,
	}
	curr.Name = canonNet.String()
	curr.Data = data

	if isNew {
		atomic.AddInt32(&t.elements, 1)
	}
	return isNew
}

// InsertCIDRWithData parses a CIDR string and inserts it with an associated data payload.
func (t *Tree) InsertCIDRWithData(cidr string, data interface{}) error {
	if t == nil {
		return errors.New("tree is nil")
	}
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("parsing cidr %q: %w", cidr, err)
	}
	t.InsertWithData(ipnet, data)
	return nil
}

// Lpm performs a Longest Prefix Match in the Tree for a net.IP (IPv4 or IPv6).
// Returns the most specific matching *net.IPNet or an error if no match exists.
func (t *Tree) Lpm(ip net.IP) (*net.IPNet, error) {
	matchNet, _, err := t.LpmWithData(ip)
	return matchNet, err
}

// PrefixLpm implements Longest Prefix Match for an IPNet's network address.
func (t *Tree) PrefixLpm(n *net.IPNet) (*net.IPNet, error) {
	if n == nil {
		return nil, errors.New("cannot LPM a nil prefix")
	}
	return t.Lpm(n.IP)
}

// LpmWithData performs LPM and returns both the matching *net.IPNet and its associated data payload.
func (t *Tree) LpmWithData(ip net.IP) (*net.IPNet, interface{}, error) {
	if t == nil {
		return nil, nil, errors.New("tree is nil")
	}
	if ip == nil {
		return nil, nil, errors.New("cannot LPM a nil IP")
	}
	rawBytes, maxBits, isV6, err := splitIP(ip)
	if err != nil {
		return nil, nil, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	var root *Node
	if isV6 {
		root = t.v6
	} else {
		root = t.v4
	}
	if root == nil {
		return nil, nil, fmt.Errorf("no match found for IP: %v", ip)
	}

	var bestNode *Node
	if root.Prefix != nil {
		bestNode = root
	}

	curr := root
	for bit := 0; bit < maxBits; bit++ {
		b := getBit(rawBytes, bit)
		if b == 0 {
			curr = curr.l
		} else {
			curr = curr.r
		}
		if curr == nil {
			break
		}
		if curr.Prefix != nil {
			bestNode = curr
		}
	}

	if bestNode != nil && bestNode.Prefix != nil {
		return bestNode.Prefix.Network, bestNode.Data, nil
	}
	return nil, nil, fmt.Errorf("no match found for IP: %v", ip)
}

// Search performs a longest prefix match search on a Node subtree for an IP address.
func (n *Node) Search(ip net.IP) (*net.IPNet, error) {
	if n == nil {
		return nil, errors.New("node is nil")
	}
	if ip == nil {
		return nil, errors.New("ip to search is nil")
	}

	rawBytes, maxBits, _, err := splitIP(ip)
	if err != nil {
		return nil, err
	}

	var bestMatch *net.IPNet
	if n.Prefix != nil && n.Prefix.Network != nil && n.Prefix.Network.Contains(ip) {
		bestMatch = n.Prefix.Network
	}

	curr := n
	for bit := 0; bit < maxBits && curr != nil; bit++ {
		b := getBit(rawBytes, bit)
		if b == 0 {
			curr = curr.l
		} else {
			curr = curr.r
		}
		if curr != nil && curr.Prefix != nil && curr.Prefix.Network != nil {
			if curr.Prefix.Network.Contains(ip) {
				bestMatch = curr.Prefix.Network
			}
		}
	}

	if bestMatch != nil {
		return bestMatch, nil
	}
	return nil, fmt.Errorf("no matching prefix found for IP: %v", ip)
}

// Find searches for an exact match of the given IPNet in the tree.
// Returns the matching *Node and true if found, or nil and false if not found.
func (t *Tree) Find(n *net.IPNet) (*Node, bool) {
	if t == nil {
		return nil, false
	}
	_, _, rawBytes, prefixLen, isV6, err := parseAndCanonicalize(n)
	if err != nil {
		return nil, false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	var root *Node
	if isV6 {
		root = t.v6
	} else {
		root = t.v4
	}
	if root == nil {
		return nil, false
	}

	curr := root
	for bit := 0; bit < prefixLen; bit++ {
		b := getBit(rawBytes, bit)
		if b == 0 {
			curr = curr.l
		} else {
			curr = curr.r
		}
		if curr == nil {
			return nil, false
		}
	}

	if curr.Prefix != nil {
		return curr, true
	}
	return nil, false
}

// FindCIDR finds an exact match for the given CIDR string.
func (t *Tree) FindCIDR(cidr string) (*Node, bool) {
	if t == nil {
		return nil, false
	}
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, false
	}
	return t.Find(ipnet)
}

// Contains checks if the IP matches any covering prefix stored in the tree.
func (t *Tree) Contains(ip net.IP) bool {
	if t == nil {
		return false
	}
	_, err := t.Lpm(ip)
	return err == nil
}

// ContainsCIDR checks if the exact CIDR prefix is present in the tree.
func (t *Tree) ContainsCIDR(cidr string) bool {
	if t == nil {
		return false
	}
	_, found := t.FindCIDR(cidr)
	return found
}

// CoveringPrefix returns the most specific prefix in the tree that encloses/covers the given network.
func (t *Tree) CoveringPrefix(n *net.IPNet) (*net.IPNet, error) {
	if t == nil {
		return nil, errors.New("tree is nil")
	}
	_, _, rawBytes, prefixLen, isV6, err := parseAndCanonicalize(n)
	if err != nil {
		return nil, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	var root *Node
	if isV6 {
		root = t.v6
	} else {
		root = t.v4
	}
	if root == nil {
		return nil, fmt.Errorf("no covering prefix found for: %v", n)
	}

	var bestNode *Node
	if root.Prefix != nil {
		bestNode = root
	}

	curr := root
	for bit := 0; bit < prefixLen; bit++ {
		b := getBit(rawBytes, bit)
		if b == 0 {
			curr = curr.l
		} else {
			curr = curr.r
		}
		if curr == nil {
			break
		}
		if curr.Prefix != nil {
			bestNode = curr
		}
	}

	if bestNode != nil && bestNode.Prefix != nil {
		return bestNode.Prefix.Network, nil
	}
	return nil, fmt.Errorf("no covering prefix found for: %v", n)
}

// CoveringPrefixes returns all prefixes in the tree that enclose/cover the given network,
// ordered from least specific to most specific.
func (t *Tree) CoveringPrefixes(n *net.IPNet) []*net.IPNet {
	if t == nil {
		return nil
	}
	_, _, rawBytes, prefixLen, isV6, err := parseAndCanonicalize(n)
	if err != nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	var root *Node
	if isV6 {
		root = t.v6
	} else {
		root = t.v4
	}
	if root == nil {
		return nil
	}

	var result []*net.IPNet
	if root.Prefix != nil {
		result = append(result, root.Prefix.Network)
	}

	curr := root
	for bit := 0; bit < prefixLen; bit++ {
		b := getBit(rawBytes, bit)
		if b == 0 {
			curr = curr.l
		} else {
			curr = curr.r
		}
		if curr == nil {
			break
		}
		if curr.Prefix != nil {
			result = append(result, curr.Prefix.Network)
		}
	}

	return result
}

// Subnets returns all prefixes in the tree that are subnets of (contained within) the given network.
func (t *Tree) Subnets(n *net.IPNet) []*net.IPNet {
	if t == nil {
		return nil
	}
	_, _, rawBytes, prefixLen, isV6, err := parseAndCanonicalize(n)
	if err != nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	var root *Node
	if isV6 {
		root = t.v6
	} else {
		root = t.v4
	}
	if root == nil {
		return nil
	}

	curr := root
	for bit := 0; bit < prefixLen; bit++ {
		b := getBit(rawBytes, bit)
		if b == 0 {
			curr = curr.l
		} else {
			curr = curr.r
		}
		if curr == nil {
			return nil
		}
	}

	var result []*net.IPNet
	var collect func(node *Node)
	collect = func(node *Node) {
		if node == nil {
			return
		}
		if node.Prefix != nil && node.Prefix.Network != nil {
			result = append(result, node.Prefix.Network)
		}
		collect(node.l)
		collect(node.r)
	}
	collect(curr)
	return result
}

// Delete removes a prefix from the tree and prunes unneeded empty branching nodes.
// Returns true if the prefix was found and removed, false otherwise.
func (t *Tree) Delete(n *net.IPNet) bool {
	if t == nil {
		return false
	}
	_, _, rawBytes, prefixLen, isV6, err := parseAndCanonicalize(n)
	if err != nil {
		return false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	var root *Node
	if isV6 {
		root = t.v6
	} else {
		root = t.v4
	}
	if root == nil {
		return false
	}

	curr := root
	for bit := 0; bit < prefixLen; bit++ {
		b := getBit(rawBytes, bit)
		if b == 0 {
			curr = curr.l
		} else {
			curr = curr.r
		}
		if curr == nil {
			return false
		}
	}

	if curr.Prefix == nil {
		return false
	}

	curr.Prefix = nil
	curr.Name = ""
	curr.Data = nil
	atomic.AddInt32(&t.elements, -1)

	// Prune unused nodes upward
	for curr != root && curr.Prefix == nil && curr.l == nil && curr.r == nil {
		parent := curr.parent
		if parent == nil {
			break
		}
		if parent.l == curr {
			parent.l = nil
		} else if parent.r == curr {
			parent.r = nil
		}
		curr = parent
	}

	return true
}

// DeleteCIDR parses a CIDR string and removes the prefix from the tree.
func (t *Tree) DeleteCIDR(cidr string) error {
	if t == nil {
		return errors.New("tree is nil")
	}
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("parsing cidr %q: %w", cidr, err)
	}
	if !t.Delete(ipnet) {
		return fmt.Errorf("prefix %s not found in tree", cidr)
	}
	return nil
}

// Size returns the total number of prefixes stored in the tree.
func (t *Tree) Size() int {
	if t == nil {
		return 0
	}
	return int(atomic.LoadInt32(&t.elements))
}

// Prefixes returns all stored prefixes across both IPv4 and IPv6 trees.
func (t *Tree) Prefixes() []*net.IPNet {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*net.IPNet
	var collect func(node *Node)
	collect = func(node *Node) {
		if node == nil {
			return
		}
		if node.Prefix != nil && node.Prefix.Network != nil {
			result = append(result, node.Prefix.Network)
		}
		collect(node.l)
		collect(node.r)
	}
	collect(t.v4)
	collect(t.v6)
	return result
}

// Walk traverses all nodes in the tree that have a stored prefix and calls fn.
// If fn returns false, iteration stops.
func (t *Tree) Walk(fn func(n *Node) bool) {
	if t == nil {
		return
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	var walk func(node *Node) bool
	walk = func(node *Node) bool {
		if node == nil {
			return true
		}
		if node.Prefix != nil {
			if !fn(node) {
				return false
			}
		}
		if !walk(node.l) {
			return false
		}
		return walk(node.r)
	}

	if !walk(t.v4) {
		return
	}
	walk(t.v6)
}
