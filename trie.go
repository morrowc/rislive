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
	Root     *Node
	elements int
}

// Node is a single tree element, with linkage to it's parent and 2 siblings.
type Node struct {
	Name   string
	Prefix *net.IPNet
	parent *Node
	l, r   *Node
	lock   *sync.Mutex
}

func New() *Tree {
	return &Tree{
		Root:     &Node{},
		elements: 1,
	}
}

// Lpm performs a longest prefix match in a Tree for a IPNet.
// The match is returned or an error if there is no match.
func (t *Tre) Lpm(n *net.IPNet) (*net.IPNet, error) {
	return nil, fmt.Errorf("failed to find match for: %v", n)
}

// Insert adds a prefix to the tree, provided the prefix doesn't already exist in the tree.
func (t *Tree) Insert(n *net.IPNet) bool {
	return true
}
