// Add a simple trie to be used in longest prefix matching for
// ip prefixes/networks.
package main

import (
	"fmt"
	"net"
	"sync"
)

// Tree is the binary (trie) tree which stores preefixes.
type Tree struct {
	Root     []*Node // The first octet or nibble of an address.
	elements int     // total number of elements stored in the tree.
}

// Node is a single tree element, with linkage to it's parent and 2 siblings.
type Node struct {
	Name   string      // A nexthop.
	Prefix *net.IPNet  // The prefix for this node.
	parent *Node       // The node to which this node attaches.
	l, r   *Node       // The nodes which attach to this node.
	lock   *sync.Mutex // A mutex, to permit locking the structure if changes are to be made.
}

func (n *Node) Search(net.IPNet) *net.IPNet {
	return nil
}

func New() *Tree {
	return &Tree{
		Root:     make([]*Node, 256),
		elements: 1,
	}
}

// Lpm performs a longest prefix match in a Tree for a IPNet.
// The match is returned or an error if there is no match.
func (t *Tree) Lpm(n *net.IPNet) (*net.IPNet, error) {
	if n == nil {
		return nil, fmt.Errorf("can not LPM a nil prefix: %v", n)
	}

	// Extract the first byte/octet from prefix being searched for.
	o := n.IP[0]

	// If the first Octet exists, keep searching for the best match.
	if node := t.Root[o]; node.Prefix != nil {
		fmt.Printf("found first level match(%v) for: %v\n", n, o)
		return node.Search(n), nil
	}

	return nil, fmt.Errorf("failed to find match for: %v", n)
}

// Insert adds a prefix to the tree, provided the prefix doesn't already exist in the tree.
func (t *Tree) Insert(n *net.IPNet) bool {
	return true
}
