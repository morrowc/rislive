// A patricia trie library for ipv4 or ipv6 storage/lookup.
package trie2

import (
	"bytes"
	"fmt"
	"net"
)

// Determine the address-family for a single
func af(ip net.IP) int {
	if four := ip.To4(); four != nil {
		return 4
	}
	return 6
}

// sameFamily returns true of a && b are the same addres family.
func sameFamily(a, b *net.IPNet) bool {
	aAF := af(a.IP)
	bAF := af(b.IP)
	if aAF != bAF {
		return false
	}
	return true
}

// PatriciaTrie is a patricia trie data structure for IPv4 and IPv6 addresses.
type PatriciaTrie struct {
	root *Node
}

// Node is a node in a patricia trie.
type Node struct {
	children [256]*Node
	isLeaf   bool
	ip       *net.IPNet
}

// NewPatriciaTrie creates a new patricia trie.
func NewPatriciaTrie() *PatriciaTrie {
	return &PatriciaTrie{
		root: &Node{},
	}
}

// Insert inserts an IPv4 or IPv6 address into the patricia trie.
// Insert should lookup the longest match node and insert at that point
// in the Trie.
func (t *PatriciaTrie) Insert(ip *net.IPNet) {
	node := t.root
	for _, b := range ip.IP {
		if node.children[b] == nil {
			node.children[b] = &Node{}
		}
		node = node.children[b]
	}
	node.isLeaf = true
}

// Lookup looks up an IPv4 or IPv6 address in the patricia trie.
func (trie *PatriciaTrie) Lookup(ip *net.IPNet) *Node {
	// return trie.lookup(trie.root, ip)
	return nil
}

func (trie *PatriciaTrie) lookup(node *Node, ip *net.IPNet) *Node {
	if node == nil {
		return nil
	}

	// Verify that node.ip and ip are of the same family.
	if !sameFamily(node.ip, ip) {
		return nil
	}

	// Check if the IP address matches the node.
	// Determine if ip.IP is contained in node.ip, and if ip.IPMask , if so, then ip
	if bytes.Equal(node.ip.IP, ip.IP) {
		return node
	}

	// Check if the IP address is a prefix of the node.
	if node.isLeaf {
		return nil
	}

	// Recursively lookup the IP address in the child node.
	return trie.lookup(node.children[ip[0]], ip)
}

// GetNetblock returns the node in the trie that represents the netblock.
func (t *PatriciaTrie) GetNetblock(netblock *net.IPNet) *Node {
	node := t.root
	for _, b := range netblock.IP {
		if node.children[b] == nil {
			return nil
		}
		node = node.children[b]
	}
	return node
}

// DeleteNetblock deletes a netblock from the trie.
func (t *PatriciaTrie) DeleteNetblock(netblock *net.IPNet) error {
	node := t.GetNetblock(netblock)
	if node == nil {
		return fmt.Errorf("Netblock %s not found", netblock.String())
	}
	node.isLeaf = false
	return nil
}

/*
func (t *PatriciaTrie) String() string {
	var sb strings.Builder
	sb.WriteString("{")
	node := t.root
	for node != nil {
		if node.isLeaf {
			sb.WriteString(node.IP.String())
		} else {
			for _, b := range node.IP {
				sb.WriteByte(b)
			}
			sb.WriteString(", ")
		}
		node = node.children[0]
	}
	sb.WriteString("}")
	return sb.String()
}
*/
