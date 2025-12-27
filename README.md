# Go Raft Consensus: Educational Implementation

![Go Version](https://img.shields.io/badge/go-1.22%2B-00ADD8?style=for-the-badge&logo=go)
![License](https://img.shields.io/badge/license-MIT-green?style=for-the-badge)
![Status](https://img.shields.io/badge/status-active-success?style=for-the-badge)
![Maintenance](https://img.shields.io/badge/maintenance-educational-orange?style=for-the-badge)

A clean, thread-safe, and modular implementation of the Raft Consensus Algorithm written in pure Go. This project is designed for students, engineers, and enthusiasts who want to understand the core mechanics of distributed consensus—specifically Leader Election and Heartbeat management—without the complexity of production-grade storage or RPC frameworks.

---

## Table of Contents

1. [Overview](#overview)
2. [Key Features](#key-features)
3. [Project Architecture](#project-architecture)
4. [The Raft Protocol: Under the Hood](#the-raft-protocol-under-the-hood)
    - [Node States](#node-states)
    - [Leader Election Process](#leader-election-process)
    - [Heartbeats & Authority](#heartbeats--authority)
5. [Technical Details](#technical-details)
    - [Concurrency Model](#concurrency-model)
    - [In-Memory Transport](#in-memory-transport)
6. [Installation & Usage](#installation--usage)
7. [Configuration](#configuration)
8. [Roadmap](#roadmap)
9. [License](#license)

---

## Overview

Raft is a consensus algorithm that is designed to be easy to understand. It is equivalent to Paxos in fault-tolerance and performance. The primary goal of this repository is to demonstrate how nodes in a distributed system can agree on who the leader is, even in the face of network partitions or delays (simulated).

This implementation focuses on the Control Plane of Raft:
* How nodes vote.
* How terms are incremented.
* How a leader asserts authority.
* How split votes are resolved via randomized timeouts.

Unlike full-scale implementations like etcd or Consul, this project strips away the log storage and snapshotting layers to expose the raw state machine logic driving the cluster.

---

## Key Features

* **Finite State Machine (FSM):** Rigorous implementation of the three Raft states (Follower, Candidate, Leader) with strict transition rules.
* **Randomized Election Timeouts:** Prevents "split votes" where nodes constantly restart elections simultaneously, ensuring the system converges quickly.
* **Heartbeat Mechanism:** Leaders send periodic `MsgAppendEntries` messages to suppress new elections and maintain cluster stability.
* **Thread-Safe Design:** Extensive use of `sync.Mutex` and Go channels to prevent race conditions during state transitions.
* **Modular Codebase:** Clean separation of concerns between Transport (Network), Types (Data), and Logic (Node).
* **Simulation Ready:** Includes a bootstrap entry point that spins up a 3-node cluster locally for immediate observation.

---

## Project Architecture

The codebase is refactored into logical components to mimic a production-grade structure, making it easier to read and extend.

```text
.
├── go.mod                # Go module definition
├── main.go               # Entry point: Bootstraps the cluster and handles shutdown
├── node.go               # CORE LOGIC: The Raft state machine and event loops
├── transport.go          # NETWORK: Simulates message passing via buffered channels
└── types.go              # DATA: Struct definitions (Message, State) and Enums

```

### File Breakdown

* **`types.go`**: Defines the vocabulary of the system. It contains the `Message` struct which mimics an RPC packet, and the `State` constants.
* **`transport.go`**: An abstraction layer for networking. Instead of TCP/UDP, it uses Go's buffered channels to simulate network traffic. This allows for deterministic testing and easy debugging without network stack overhead.
* **`node.go`**: The heart of the application. It contains the `Node` struct and the methods corresponding to the Raft paper's RPC handlers (`RequestVote`, `AppendEntries`) and the main run loop.
* **`main.go`**: Orchestrates the creation of the cluster, wiring the transport to the nodes and managing the lifecycle of the goroutines.

---

## The Raft Protocol: Under the Hood

Raft achieves consensus by first electing a distinguished leader, then giving the leader complete responsibility for managing the replicated log. This repository implements the first and most critical phase: Leader Election.

### Node States

At any given time, each server is in one of three states:

1. **Follower**:
* Passive state. Issues no requests on its own but simply responds to requests from leaders and candidates.
* If a follower receives no communication (Heartbeats) over a period of time called the *election timeout*, it assumes there is no viable leader and begins an election to choose a new one.


2. **Candidate**:
* Active state used to elect a new leader.
* A candidate votes for itself and issues `RequestVote` RPCs in parallel to each of the other servers in the cluster.
* It remains in this state until it wins the election, another server establishes itself as leader, or a period of time goes by with no winner.


3. **Leader**:
* Active state handling all client requests (not implemented in this demo) and coordinating the cluster.
* Sends periodic heartbeats (`AppendEntries` with empty logs) to all followers to maintain authority and prevent new elections.



### Leader Election Process

The election mechanism is driven by the `runCandidate` loop in `node.go`.

1. **Timeout**: When a Follower's election timeout expires, it increments its current term and transitions to Candidate state.
2. **Self-Vote**: The node votes for itself.
3. **Broadcast**: It sends `MsgRequestVote` to all peers.
4. **Counting**:
* If it receives votes from a majority of the servers (N/2 + 1), it becomes the Leader.
* If it receives an `AppendEntries` RPC from another server claiming to be leader with a term >= current term, it accepts the leader and returns to Follower state.
* If the election timeout elapses without a winner (split vote), it increments the term and starts a new election.



### Heartbeats & Authority

Once elected, the Leader executes the `runLeader` loop.

* It immediately sends a heartbeat to assert authority.
* It sets up a `time.Ticker` to send heartbeats at fixed intervals (typically much shorter than the election timeout).
* Followers reset their election timeouts every time they receive a valid heartbeat.
* This mechanism ensures that as long as the leader is alive and reachable, no other node will attempt to take over.

---

## Technical Details

### Concurrency Model

Go is particularly well-suited for distributed systems due to its lightweight threads (Goroutines).

* **Node Loop**: Each `Node` runs in its own goroutine (`Run` method). This simulates an independent server process.
* **State Locking**: A `sync.Mutex` is used whenever accessing shared state (like `currentTerm` or `votedFor`) to ensure memory safety.
* **Channels**: Communication between nodes is handled via channels. The `Recv()` method returns a read-only channel that the node listens to in a `select` block. This avoids the need for complex callback logic or manual thread management.

### In-Memory Transport

The `transport.go` file implements a `LocalTransport` struct.

* It maintains a map of `id -> chan Message`.
* The `Send` method looks up the recipient's channel and pushes the message.
* To simulate network buffering, the channels have a capacity (e.g., 64 messages).
* This abstraction allows the core Raft logic to remain agnostic of the underlying transport layer. In a future iteration, this could be replaced with gRPC or HTTP transport without changing `node.go`.

---

## Installation & Usage

### Prerequisites

* Go 1.22 or higher.

### Steps to Run

1. **Clone the repository:**
```bash
git clone [https://github.com/HamzaEnti/go-raft-consensus.git](https://github.com/HamzaEnti/go-raft-consensus.git)
cd go-raft-consensus

```


2. **Build the project:**
```bash
go build -o raft-node

```


3. **Run the demo:**
The default `main.go` configuration launches a 3-node cluster that runs for 10 seconds.
```bash
go run .

```



### Expected Output

You will see logs indicating the state transitions of the nodes. Look for:

* `[n0 t=0] starting as Follower`
* `[n1 t=1] election timeout, switching to Candidate`
* `[n1 t=2] Majority achieved (2 votes), becoming Leader`
* `[n0 t=2] voted for 1 at term 2`

---

## Configuration

Currently, configuration is defined as constants within the code for simplicity.

* **Number of Nodes**: Defined in `main.go` (`const numNodes = 3`).
* **Election Timeout**: Defined in `node.go` (`randomElectionTimeout`), typically 300-600ms.
* **Heartbeat Interval**: Defined in `node.go` (`runLeader`), typically 100ms.

To change these values, modify the source code and re-run.

---

## Roadmap

This project is intended to grow. Future planned features include:

* [ ] **Log Replication**: Implementing the actual log entries and the `Log` slice in the Node struct.
* [ ] **Persistence**: Saving `currentTerm`, `votedFor`, and `log` to stable storage (disk) to survive crashes.
* [ ] **RPC Transport**: Replacing channels with a real network transport using Go `net/rpc` or gRPC.
* [ ] **Cluster Membership Changes**: Allowing nodes to be added or removed dynamically.
* [ ] **Unit Tests**: Adding comprehensive tests for state transitions and edge cases.

---

## License

This project is licensed under the MIT License. See the LICENSE file for details.
