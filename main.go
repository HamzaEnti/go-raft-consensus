package main

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

func main() {
	// Seed random number generator
	rand.Seed(time.Now().UnixNano())

	const numNodes = 3

	// Shared transport
	tr := NewLocalTransport(numNodes)

	// Create nodes
	nodes := make([]*Node, 0, numNodes)
	for id := 0; id < numNodes; id++ {
		var peers []int
		for j := 0; j < numNodes; j++ {
			if j != id {
				peers = append(peers, j)
			}
		}
		nodes = append(nodes, NewNode(id, peers, tr))
	}

	// Context to stop all nodes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(numNodes)
	for _, n := range nodes {
		go n.Run(ctx, &wg)
	}

	// Let the cluster run for 10 seconds
	time.Sleep(10 * time.Second)

	// Stop the cluster
	cancel()
	wg.Wait()
}
