package client

import (
	"context"
	"errors"
	"grpcvsthrift/thrift/client/pool"
	"grpcvsthrift/thrift/rpc"
	"sync"
	"time"
)

const (
	Up = iota
	Down
	Unknown
)

type Conn struct {
	sync.RWMutex
	state       int32
	addr        string
	idleTimeout time.Duration
	connTimeout time.Duration
	lastPing    time.Time
	lastStatus  bool
	checkClient *pool.IdleClient //Ping专用
	ThriftPool  *pool.ThriftPool
}

func newConn(addr string, connTimeout, idleTimeout time.Duration) (*Conn, error) {

	conn := &Conn{
		connTimeout: connTimeout,
		idleTimeout: idleTimeout,
		addr:        addr,
		state:       Unknown,
		lastPing:    time.Now().UTC().Add(8 * time.Hour),
		lastStatus:  true,

		ThriftPool: pool.NewThriftPool(addr, 10000, connTimeout, idleTimeout, thriftDialFunc, thriftClientCloseFunc),
	}

	//初始化校验Client
	checkClient, err := thriftDialFunc(addr, conn.connTimeout)
	if err != nil {
		return nil, err
	}
	conn.checkClient = checkClient

	idleClient, err := conn.ThriftPool.Get()
	if err != nil {
		conn.Close()
		return nil, err
	}

	conn.ThriftPool.Put(idleClient)

	return conn, nil
}

//如果Ping超时了，那么把Thrift pool里的资源全部释放掉
func (conn *Conn) Release() {
	conn.Lock()
	if conn.checkClient != nil {
		thriftClientCloseFunc(conn.checkClient)
	}
	if conn.ThriftPool != nil {
		conn.ThriftPool.Release()
	}
	conn.lastStatus = false
	conn.checkClient = nil
	conn.Unlock()
}

//这里仅仅是将连接关闭
func (conn *Conn) Close() {
	conn.Lock()
	if conn.checkClient != nil {
		thriftClientCloseFunc(conn.checkClient)
	}
	if conn.ThriftPool != nil {
		conn.ThriftPool.Release()
	}
	conn.checkClient = nil
	conn.ThriftPool = nil
	conn.lastStatus = false
	conn.Unlock()
}

func (conn *Conn) SetLastPing() {
	conn.lastPing = time.Now().UTC().Add(8 * time.Hour)
}

func (conn *Conn) Addr() string {
	return conn.addr
}

func (conn *Conn) State() string {
	var state string
	switch conn.state {
	case Up:
		state = "up"
	case Down:
		state = "down"
	case Unknown:
		state = "unknown"
	}
	return state
}

func (conn *Conn) IdleConnCount() uint32 {
	if conn.ThriftPool != nil {
		return conn.ThriftPool.GetIdleCount()
	}
	return 0
}

func (conn *Conn) EnableConnCount() int32 {
	if conn.ThriftPool != nil {
		return conn.ThriftPool.GetConnCount()
	}
	return 0
}

func (conn *Conn) GetLastPing() time.Time {
	return conn.lastPing
}

func (conn *Conn) Ping(readTimeout time.Duration) (error, bool) {
	var err error
	if conn.checkClient == nil || conn.lastStatus == false {
		if conn.checkClient != nil {
			thriftClientCloseFunc(conn.checkClient)
			conn.checkClient = nil
		}
		conn.checkClient, err = thriftDialFunc(conn.addr, conn.connTimeout)
		if err != nil {
			return err, false
		}
	}

	if conn.checkClient.Transport != nil {
		if conn.checkClient.Transport.IsOpen() == false {
			return errors.New("Socket is close"), false
		}
	}

	//服务端连接测试的接口
	ch := make(chan *rpc.Health, 1)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()

	go func(ch chan *rpc.Health, errCh chan error) {
		res, err := conn.checkClient.Client.Ping(ctx)
		if err != nil {
			errCh <- err
		} else {
			ch <- res
		}
	}(ch, errCh)

	select {
	case res := <-ch:
		if res.OK == false {
			return errors.New(res.Errors), false
		}
	case err := <-errCh:
		return err, false
	case <-ctx.Done():
		return errors.New("Ping Timeout"), true
	}

	return nil, false
}
