// Add a simple trie to be used in longest prefix matching for
// ip prefixes/networks.
//
// TODO(morrowc): this should be moved to an external package.
// Additionally, more documentation for this library would be helpful.
package main

import (
	"fmt"
	"net"
	"sync"
)

// Tree is the binary (trie) tree which stores preefixes.
type Tree struct {
	Root     *Node // The top level, least specific, prefix in the tree.
	elements int32 // total number of elements stored in the tree.
}

// Prefix is a single Node's prefix, the IP (192.168.0.0/32) and Network (192.168.0.0/16).
type Prefix struct {
	IP      net.IP
	Network *net.IPNet
}

// Convenient functions to return elements of the Prefix struct.
func (p *Prefix) IP() net.IP      { return p.IP }
func (p *Prefix) Net() *net.IPNet { return p.Network }

// Node is a single tree element, with linkage to it's parent and 2 siblings.
type Node struct {
	Name   string      // A nexthop.
	Prefix *Prefix     // The prefix information for this node, IP and Network.
	parent *Node       // The node to which this node attaches.
	l, r   *Node       // The nodes which attach to this node.
	lock   *sync.Mutex // A mutex, to permit locking the structure if changes are to be made.
}

// New creates a new tree rooted at the root prefix.
func New(root string) (*Tree, error) {
	ip, net, err := net.ParseCIDR(root)
	if err != nil {
		return nil, fmt.Errorf("parsing cidr: %v failed: %v", root, err)
	}

	return &Tree{
		Root: &Node{Name: root,
			Prefix: &Prefix{IP: ip,
				Network: net},
		},
		elements: 1,
	}, nil
}

// PrefixLpm implements a Longest Prefix Match for a prefix in the LPM tree.
func (t *Tree) PrefixLpm(n *net.IPNet) (*net.IPNet, error) {
	return t.Lpm(n.IP)
}

// Lpm performs a longest prefix match in a Tree for a net.IP.
// Matching is done recursively down the L/R sides of each fork in the tree
// until neither L nor R forks match the request.
//
// The match is returned or an error if there is no match.
func (t *Tree) Lpm(n *net.IP) (*net.IPNet, error) {
	if n == nil {
		return nil, fmt.Errorf("can not LPM a nil prefix: %v", n)
	}

	// Search the L/R legs of the tree, If the L and R legs this node is the match.
	// Search down the L tree leg.

	// Search down the R tree leg.

	return nil, fmt.Errorf("failed to find match for: %v", n)
}

// Insert adds a prefix to the tree, provided the prefix doesn't already exist in the tree.
func (t *Tree) Insert(n *net.IPNet) bool {
	return true
}
