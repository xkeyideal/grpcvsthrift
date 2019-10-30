package messagequeue

import (
	pb "grpcvsthrift/grpc/rpcpb"
	"sync"
)

type EntryQueue struct {
	size        uint64
	left        []*pb.PutRequest
	right       []*pb.PutRequest
	leftInWrite bool
	stopped     bool
	idx         uint64
	mu          sync.Mutex
}

func NewEntryQueue(size uint64) *EntryQueue {
	e := &EntryQueue{
		size:  size,
		left:  make([]*pb.PutRequest, size),
		right: make([]*pb.PutRequest, size),
	}
	return e
}

func (q *EntryQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.stopped = true
}

func (q *EntryQueue) targetQueue() []*pb.PutRequest {
	var t []*pb.PutRequest
	if q.leftInWrite {
		t = q.left
	} else {
		t = q.right
	}
	return t
}

func (q *EntryQueue) Add(ent *pb.PutRequest) (bool, bool) {
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

func (q *EntryQueue) Get() []*pb.PutRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	sz := q.idx
	q.idx = 0
	t := q.targetQueue()
	q.leftInWrite = !q.leftInWrite
	return t[:sz]
}
