package pool

import (
	"container/list"
	"errors"
	"grpcvsthrift/thrift/rpc"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

const (
	CHECKINTERVAL = 120 //清除超时连接间隔

	poolOpen = 1
	poolStop = 2
)

type ThriftDial func(addr string, connTimeout time.Duration) (*IdleClient, error)

type ThriftClientClose func(c *IdleClient) error

type ThriftPool struct {
	Dial        ThriftDial
	Close       ThriftClientClose
	lock        *sync.Mutex
	idle        list.List
	idleTimeout time.Duration
	connTimeout time.Duration
	maxConn     int32
	count       int32
	addr        string
	status      uint32
}

type IdleClient struct {
	Transport thrift.TTransport
	Client    *rpc.KVClient
	Addr      string
}

type idleConn struct {
	c *IdleClient
	t time.Time
}

var nowFunc = time.Now

//error
var (
	ErrOverMax          = errors.New("ThriftPool 连接超过设置的最大连接数")
	ErrInvalidConn      = errors.New("ThriftPool Client回收时变成nil")
	ErrPoolClosed       = errors.New("ThriftPool 连接池已经被关闭")
	ErrSocketDisconnect = errors.New("ThriftPool 客户端socket连接已断开")
)

func NewThriftPool(addr string,
	maxConn int32, connTimeout, idleTimeout time.Duration,
	dial ThriftDial, closeFunc ThriftClientClose) *ThriftPool {

	thriftPool := &ThriftPool{
		Dial:        dial,
		Close:       closeFunc,
		addr:        addr,
		lock:        &sync.Mutex{},
		maxConn:     maxConn,
		idleTimeout: idleTimeout,
		connTimeout: connTimeout,
		status:      poolOpen,
		count:       0,
	}

	go thriftPool.ClearConn()

	return thriftPool
}

func (p *ThriftPool) Get() (*IdleClient, error) {

	if atomic.LoadUint32(&p.status) == poolStop {
		return nil, ErrPoolClosed
	}
	
	p.lock.Lock()
	// 判断是否超额
	if p.idle.Len() == 0 && p.count >= p.maxConn {
		p.lock.Unlock()
		return nil, ErrOverMax
	}
	
	if p.idle.Len() == 0 {
		p.lock.Unlock()

		client, err := p.Dial(p.addr, p.connTimeout)
		if err != nil {
			return nil, err
		}

		if !client.Check() {
			return nil, ErrSocketDisconnect
		}

		atomic.AddInt32(&p.count, 1)
		return client, nil
	}

	ele := p.idle.Front()
	idlec := ele.Value.(*idleConn)
	p.idle.Remove(ele)
	p.lock.Unlock()

	if !idlec.c.Check() {
		atomic.AddInt32(&p.count, -1)
		return nil, ErrSocketDisconnect
	}
	return idlec.c, nil
}

func (p *ThriftPool) Put(client *IdleClient) error {
	if client == nil {
		atomic.AddInt32(&p.count, -1)
		return ErrInvalidConn
	}

	if atomic.LoadUint32(&p.status) == poolStop {
		err := p.Close(client)
		client = nil
		return err
	}

	if atomic.LoadInt32(&p.count) > p.maxConn || !client.Check() {
		atomic.AddInt32(&p.count, -1)
		err := p.Close(client)
		client = nil
		return err
	}

	if !client.Check() {
		atomic.AddInt32(&p.count, -1)
		err := p.Close(client)
		client = nil
		return err
	}

	p.lock.Lock()
	p.idle.PushBack(&idleConn{
		c: client,
		t: nowFunc(),
	})
	p.lock.Unlock()

	return nil
}

func (p *ThriftPool) CloseErrConn(client *IdleClient) {

	atomic.AddInt32(&p.count, -1)

	if client != nil {
		p.Close(client)
	}
	client = nil
	return
}

func (p *ThriftPool) CheckTimeout() {
	p.lock.Lock()
	for p.idle.Len() != 0 {
		ele := p.idle.Front()
		if ele == nil {
			break
		}
		v := ele.Value.(*idleConn)
		if v.t.Add(p.idleTimeout).After(nowFunc()) {
			break
		}
		//timeout && clear
		p.idle.Remove(ele)
		p.lock.Unlock()

		p.Close(v.c) //close client connection
		atomic.AddInt32(&p.count, -1)

		p.lock.Lock()
	}
	p.lock.Unlock()
	return
}

func (c *IdleClient) Check() bool {
	if c.Transport == nil || c.Client == nil {
		return false
	}
	return c.Transport.IsOpen()
}

func (p *ThriftPool) GetIdleCount() uint32 {
	if p != nil {
		return uint32(p.idle.Len())
	}
	return 0
}

func (p *ThriftPool) GetConnCount() int32 {
	if p != nil {
		return atomic.LoadInt32(&p.count)
	}
	return 0
}

func (p *ThriftPool) ClearConn() {
	for {
		p.CheckTimeout()
		time.Sleep(CHECKINTERVAL * time.Second)
	}
}

func (p *ThriftPool) Release() {
	atomic.StoreInt32(&p.count, 0)
	// atomic.StoreUint32(&p.status, poolStop)

	p.lock.Lock()
	idle := p.idle
	p.idle.Init()
	p.lock.Unlock()

	for iter := idle.Front(); iter != nil; iter = iter.Next() {
		p.Close(iter.Value.(*idleConn).c)
	}
}

func (p *ThriftPool) Recover() {
	atomic.StoreUint32(&p.status, poolOpen)
}
