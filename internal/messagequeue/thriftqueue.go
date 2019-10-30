package messagequeue

import (
	"grpcvsthrift/thrift/rpc"
	"sync"
)

type ThriftEntryQueue struct {
	size        uint64
	left        []*rpc.PutRequest
	right       []*rpc.PutRequest
	leftInWrite bool
	stopped     bool
	idx         uint64
	mu          sync.Mutex
}

func NewThriftEntryQueue(size uint64) *ThriftEntryQueue {
	e := &ThriftEntryQueue{
		size:  size,
		left:  make([]*rpc.PutRequest, size),
		right: make([]*rpc.PutRequest, size),
	}
	return e
}

func (q *ThriftEntryQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.stopped = true
}

func (q *ThriftEntryQueue) targetQueue() []*rpc.PutRequest {
	var t []*rpc.PutRequest
	if q.leftInWrite {
		t = q.left
	} else {
		t = q.right
	}
	return t
}

func (q *ThriftEntryQueue) Add(ent *rpc.PutRequest) (bool, bool) {
	q.mu.Lock()
	if q.idx >= q.size {
		q.mu.Unlock()
		return false, q.stopped
	}
	if q.stopped {
		q.mu.Unlock()
		return false, true
	}
	w := q.targetQueue()
	w[q.idx] = ent
	q.idx++
	q.mu.Unlock()
	return true, false
}

func (q *ThriftEntryQueue) Get() []*rpc.PutRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	sz := q.idx
	q.idx = 0
	t := q.targetQueue()
	q.leftInWrite = !q.leftInWrite
	return t[:sz]
}
