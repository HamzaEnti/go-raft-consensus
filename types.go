package main

// ----------------------------
// Basic Raft Types
// ----------------------------

type MsgType int

const (
	MsgRequestVote MsgType = iota
	MsgRequestVoteResp
	MsgAppendEntries     // Heartbeat
	MsgAppendEntriesResp // Not heavily used in this simple version
)

type Message struct {
	From        int
	To          int
	Term        int
	Type        MsgType
	VoteGranted bool // Only used in MsgRequestVoteResp
}

// Node State
type State int

const (
	StateFollower State = iota
	StateCandidate
	StateLeader
)

func (s State) String() string {
	switch s {
	case StateFollower:
		return "Follower"
	case StateCandidate:
		return "Candidate"
	case StateLeader:
		return "Leader"
	default:
		return "Unknown"
	}
}
