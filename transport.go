package main

// ----------------------------
// Local Transport (In-Memory)
// ----------------------------

// Simple channel-based transport: no packet loss or delay simulated here.
type Transport interface {
	Send(m Message)
	Recv(id int) <-chan Message
}

// LocalTransport keeps a channel for each node ID.
type LocalTransport struct {
	inbox map[int]chan Message
}

func NewLocalTransport(numNodes int) *LocalTransport {
	t := &LocalTransport{
		inbox: make(map[int]chan Message, numNodes),
	}
	for i := 0; i < numNodes; i++ {
		t.inbox[i] = make(chan Message, 64)
	}
	return t
}

func (t *LocalTransport) Send(m Message) {
	ch, ok := t.inbox[m.To]
	if !ok {
		// In a real environment, we would log this error.
		return
	}
	ch <- m
}

func (t *LocalTransport) Recv(id int) <-chan Message {
	return t.inbox[id]
}
