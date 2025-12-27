package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ----------------------------
// Raft Node
// ----------------------------

type Node struct {
	id        int
	peers     []int // List of other nodes (excluding self)
	transport Transport

	// Persistent state (simplified)
	term int

	// Candidate ID that received vote in current term
	votedFor int

	// Mutex for safe printing
	printMu sync.Mutex
}

func NewNode(id int, peers []int, tr Transport) *Node {
	return &Node{
		id:        id,
		peers:     peers,
		transport: tr,
		term:      0,
		votedFor:  -1,
	}
}

// Safe logging to avoid line mixing
func (n *Node) logf(format string, args ...any) {
	n.printMu.Lock()
	defer n.printMu.Unlock()
	prefix := fmt.Sprintf("[n%d t=%d] ", n.id, n.term)
	fmt.Printf(prefix+format+"\n", args...)
}

// Random election timeout (e.g., 300–600 ms)
func randomElectionTimeout() time.Duration {
	return time.Duration(rand.Intn(301)+300) * time.Millisecond
}

// Sends a message to all other nodes
func (n *Node) broadcastHeartbeat() {
	for _, peer := range n.peers {
		n.transport.Send(Message{From: n.id, To: peer, Term: n.term, Type: MsgAppendEntries})
	}
}

// Handler for RequestVote when in Follower state.
// Returns true if the vote was granted.
func (n *Node) handleRequestVoteFollower(m Message) bool {
	// 1. If their term is lower than mine, ignore.
	if m.Term < n.term {
		return false
	}

	// If we see a higher term, update our term and reset vote.
	if m.Term > n.term {
		n.term = m.Term
		n.votedFor = -1
	}

	voteGranted := false
	// 2. Grant vote if we haven't voted yet, or if we already voted for this candidate.
	if n.votedFor == -1 || n.votedFor == m.From {
		voteGranted = true
		n.votedFor = m.From
	}

	// 3. Send response
	n.transport.Send(Message{From: n.id, To: m.From, Term: n.term, Type: MsgRequestVoteResp, VoteGranted: voteGranted})
	return voteGranted
}

// LEADER MESSAGE HANDLING
// Handler for AppendEntries (heartbeat) when in Follower state.
// Returns true if the leader's term is accepted.
func (n *Node) handleAppendEntriesFollower(m Message) bool {
	// 1. Check leader's term
	if m.Term < n.term {
		n.transport.Send(Message{From: n.id, To: m.From, Term: n.term, Type: MsgAppendEntriesResp})
		return false
	}

	// If leader is valid, update state
	if m.Term > n.term {
		n.term = m.Term
		n.votedFor = -1
	}

	// 2. Send OK response
	n.transport.Send(Message{From: n.id, To: m.From, Term: n.term, Type: MsgAppendEntriesResp})

	return true
}

// ----------------------------
// Main Node Loop
// ----------------------------

// Run starts the main node loop, switching between states.
func (n *Node) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	state := StateFollower
	n.logf("starting as %s", state)

	for {
		if ctx.Err() != nil {
			n.logf("stopping node")
			return
		}
		switch state {
		case StateFollower:
			state = n.runFollower(ctx)
		case StateCandidate:
			state = n.runCandidate(ctx)
		case StateLeader:
			state = n.runLeader(ctx)
		default:
			n.logf("unknown state, exiting")
			return
		}
	}
}

// Follower State: waits for heartbeats or vote requests.
// If timeout passes without activity, becomes Candidate.
func (n *Node) runFollower(ctx context.Context) State {
	n.logf("→ Follower")
	for {
		select {
		case <-ctx.Done():
			return StateFollower
		case <-time.After(randomElectionTimeout()):
			n.logf("election timeout, switching to Candidate")
			return StateCandidate
		case m := <-n.transport.Recv(n.id):
			switch m.Type {
			case MsgAppendEntries: // Received heartbeat from leader
				if n.handleAppendEntriesFollower(m) {
					// If heartbeat accepted, just continue loop (resetting timeout effectively in next iteration logic depending on implementation, 
                    // though here we just process the message. Ideally, receiving a valid heartbeat should reset the timer loop).
				}
			case MsgRequestVote:
				if n.handleRequestVoteFollower(m) {
					n.logf("voted for %d at term %d", m.From, n.term)
				}
			case MsgRequestVoteResp, MsgAppendEntriesResp:
				// As follower, ignore responses.
			}
		}
	}
}

// Candidate State:
// - Increments term.
// - Votes for self.
// - Sends RequestVote to all peers.
// - Waits for majority votes or AppendEntries from a valid leader.
func (n *Node) runCandidate(ctx context.Context) State {
	n.logf("→ Candidate")

	n.term++          // Increment term
	n.votedFor = n.id // Vote for self
	votesReceived := 1
	votesNeeded := (len(n.peers)+1)/2 + 1 // Majority calculation: N/2 + 1

	// Send MsgRequestVote to all nodes with current term
	for _, peer := range n.peers {
		n.transport.Send(Message{From: n.id, To: peer, Term: n.term, Type: MsgRequestVote})
	}

	timeout := time.After(randomElectionTimeout())

	for {
		select {
		case <-ctx.Done():
			return StateFollower
		case <-timeout:
			n.logf("election finished without result, retrying")
			// Return to Candidate to start a new election
			return StateCandidate
		case m := <-n.transport.Recv(n.id):
			// If we see a higher term in any message, revert to Follower
			if m.Term > n.term {
				n.term = m.Term
				n.votedFor = -1
				return StateFollower
			}

			switch m.Type {
			case MsgRequestVoteResp:
				// Double check term
				if m.Term > n.term {
					n.term = m.Term
					n.votedFor = -1
					return StateFollower
				}

				// If response has same term and vote granted
				if m.Term == n.term && m.VoteGranted {
					votesReceived++
				}

				// If majority achieved, become Leader
				if votesReceived >= votesNeeded {
					n.logf("Majority achieved (%d votes), becoming Leader", votesReceived)
					return StateLeader
				}
			case MsgAppendEntries:
				// If a valid leader appears, revert to Follower
				if m.Term >= n.term {
					n.term = m.Term
					n.votedFor = -1
					return StateFollower
				}
			case MsgRequestVote:
				// We can respond to vote requests even as candidate (if term matches/higher logic applies)
				n.handleRequestVoteFollower(m)
			}
		}
	}
}

// Leader State: sends periodic heartbeats.
// If it receives a RequestVote or AppendEntries with a higher term, reverts to Follower.
func (n *Node) runLeader(ctx context.Context) State {
	n.logf("→ Leader (term %d)", n.term)

	// Use Ticker for repeated heartbeats
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Send immediate heartbeat upon entering state
	n.broadcastHeartbeat()

	for {
		select {
		case <-ctx.Done():
			return StateFollower

		case <-ticker.C:
			// Send periodic heartbeat
			n.broadcastHeartbeat()

		case m := <-n.transport.Recv(n.id):
			// If we see a higher term, revert to Follower
			if m.Term > n.term {
				n.term = m.Term
				n.votedFor = -1
				n.logf("seen higher term (%d), stepping down to follower", m.Term)
				return StateFollower
			}

			switch m.Type {
			case MsgRequestVote:
				// If incoming vote request has higher term, update and step down
				if m.Term > n.term {
					n.term = m.Term
					n.votedFor = -1
					n.handleRequestVoteFollower(m)
					n.logf("seen higher term (%d), stepping down to follower", m.Term)
					return StateFollower
				}
				// If term is lower or equal, handle normally (likely reject if equal and we are leader)
				n.handleRequestVoteFollower(m)

			case MsgAppendEntries:
				// If another leader appears with better/equal term, step down
				if m.Term >= n.term {
					n.term = m.Term
					n.votedFor = -1
					return StateFollower
				}

			case MsgRequestVoteResp, MsgAppendEntriesResp:
				// In this simple version, we ignore responses.
			}
		}
	}
}
