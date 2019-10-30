package client

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"log"
)

var (
	ErrConnIsNil         = errors.New("Server Conn connection is nil")
	ErrBadConn           = errors.New("Server Conn connection was bad, Thrift Pool is Nil")
	ErrAllConnsBad       = errors.New("Server Conn 所有connection is nil")
	ErrCallServerTimeout = errors.New("Server调用服务端程序超时")
	ErrServerDown        = errors.New("Server is down")
)

type Node struct {
	sync.RWMutex

	nodeAddrs     []string
	checkInterval time.Duration
	connTimeout   time.Duration
	idleTimeout   time.Duration
	readTimeout   time.Duration
	conns         []*Conn

	exitChan chan struct{}
	wg       sync.WaitGroup
}

func NewServerNode(nodeAddrs []string, checkInterval, connTimeout, idleTimeout, readTimeout time.Duration) (*Node, error) {
	n := &Node{
		nodeAddrs:     nodeAddrs,
		checkInterval: checkInterval,
		connTimeout:   connTimeout,
		idleTimeout:   idleTimeout,
		readTimeout:   readTimeout,
		conns:         []*Conn{},

		exitChan: make(chan struct{}),
		wg:       sync.WaitGroup{},
	}

	err := n.initPerConn()
	if err != nil {
		return nil, err //此处err在InitRpcConn时已经记录是哪个ip出问题了，这里不需要再记录
	}

	go n.checkNode()

	return n, nil
}

func (n *Node) Close() {
	close(n.exitChan)
	for _, conn := range n.conns {
		conn.Close()
	}
	n.wg.Wait()
}

func (n *Node) checkNode() {
	n.wg.Add(1)

	ticker := time.NewTicker(n.checkInterval)
	for {
		select {
		case <-n.exitChan:
			n.wg.Done()
			return
		case <-ticker.C:
			n.RLock()
			if n.conns == nil {
				n.RUnlock()
				continue
			}
			conns := make([]*Conn, len(n.conns))
			copy(conns, n.conns)
			n.RUnlock()
			count := len(conns)

			for i := 0; i < count; i++ {
				err, timeout := conns[i].Ping(n.readTimeout)
				if timeout { //ping 超时引起的错误，不把节点设置为down
					if atomic.LoadInt32(&(conns[i].state)) == Unknown {
						atomic.StoreInt32(&(conns[i].state), Up)
					}
					//如果超时了，那么把原来的连接全部释放掉，但并不将server标记为down，pool没有资源会重新new
					conns[i].Release()
					log.Fatalf("addr - %s ping timeout, err: %s", conns[i].addr, err.Error())
				} else {
					//如果不是超时引起的错误
					if err != nil {
						conns[i].lastStatus = false //设置最后的ping的状态为false，下次ping的时候重新申请checkClient
						//先设置为下线，然后再去释放该连接
						atomic.StoreInt32(&(conns[i].state), Down)
						n.downConn(conns[i].addr)
						log.Fatalf("addr - %s server down, err: %s", conns[i].addr, err.Error())
					} else {
						if atomic.LoadInt32(&(conns[i].state)) == Down {
							err := n.upConn(conns[i].addr)
							if err != nil {
								log.Fatalf("addr - %s server up failed, err: %s", conns[i].addr, err.Error())
							} else {
								log.Fatalf("addr - %s server up", conns[i].addr)
							}
						} else if atomic.LoadInt32(&(conns[i].state)) == Unknown {
							atomic.StoreInt32(&(conns[i].state), Up)
						}
						conns[i].lastStatus = true
					}
				}
				conns[i].SetLastPing()
			}
		}
	}
}

//需要下线Server，需关闭该Conn
func (n *Node) downConn(addr string) error {
	n.RLock()
	if n.conns == nil {
		n.RUnlock()
		return ErrAllConnsBad
	}

	conns := make([]*Conn, len(n.conns))
	copy(conns, n.conns)
	n.RUnlock()
	for _, conn := range conns {
		if conn.addr == addr {
			atomic.StoreInt32(&(conn.state), Down)
			conn.Close() //此处函数里有写锁
			break
		}
	}
	return nil
}

func (n *Node) upConn(addr string) error {
	newConn, err := newConn(addr, n.connTimeout, n.idleTimeout)
	if err != nil {
		return err
	}

	err, _ = newConn.Ping(n.readTimeout)
	if err != nil {
		atomic.StoreInt32(&(newConn.state), Down)
		newConn.Close()
		return err
	}

	atomic.StoreInt32(&(newConn.state), Up)

	n.Lock()
	for k, conn := range n.conns {
		if conn.addr == addr {
			n.conns[k] = newConn
			n.Unlock()
			return nil
		}
	}
	n.conns = append(n.conns, newConn)
	n.Unlock()
	return nil
}

func (n *Node) initPerConn() error {

	count := len(n.nodeAddrs)
	for i := 0; i < count; i++ {
		conn, err := newConn(n.nodeAddrs[i], n.connTimeout, n.idleTimeout)
		if err != nil {
			return fmt.Errorf("init addr - %s, %s", n.nodeAddrs[i], err.Error())
		}

		n.conns = append(n.conns, conn)
	}

	return nil
}

func (n *Node) getStatConn() (*Conn, error) {
	n.RLock()
	defer n.RUnlock()

	enableConnIndex := []int{}
	for i, conn := range n.conns {
		if atomic.LoadInt32(&(conn.state)) == Down {
			continue
		}
		enableConnIndex = append(enableConnIndex, i)
	}
	//该server的所有节点都down了
	if len(enableConnIndex) <= 0 {
		err := fmt.Errorf("no enable connection server")
		return nil, err
	}
	//随机获取一个S1n Server
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(enableConnIndex))
	enableConn := n.conns[enableConnIndex[index]]
	if enableConn == nil {
		return nil, ErrConnIsNil
	}

	if atomic.LoadInt32(&(enableConn.state)) == Down {
		return nil, fmt.Errorf("server addr: %s is down", enableConn.Addr())
	}

	return enableConn, nil
}

//返回一个当前可用的连接，读的过程中，可能某个conn会Down，这样能及时排除掉
func (n *Node) ChooseNodeConn() (*Conn, error) {
	n.RLock()
	defer n.RUnlock()

	enableConnIndex := []int{}
	for i, conn := range n.conns {
		if atomic.LoadInt32(&(conn.state)) == Down {
			continue
		}
		enableConnIndex = append(enableConnIndex, i)
	}
	//该server的所有节点都down了
	if len(enableConnIndex) <= 0 {
		err := fmt.Errorf("no enable connection server")
		return nil, err
	}

	//随机获取一个S1n Server
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(enableConnIndex))
	enableConn := n.conns[enableConnIndex[index]]
	if enableConn == nil {
		return nil, ErrConnIsNil
	}

	if atomic.LoadInt32(&(enableConn.state)) == Down {
		return nil, fmt.Errorf("server addr: %s is down", enableConn.Addr())
	}

	if enableConn.ThriftPool == nil {
		return nil, ErrBadConn
	}
	return enableConn, nil
}

type NodeConnsInfo struct {
	State           string    `json:"state"`
	LastPing        time.Time `json:"lastPing"`
	Addr            string    `json:"addr"`
	IdleConnCount   uint32    `json:"idleConnCount"`
	EnableConnCount int32     `json:"enableConnCount"`
}

func (n *Node) GetNodeConnsInfo() []NodeConnsInfo {
	connsInfos := []NodeConnsInfo{}
	var idleConnCount uint32
	var enableConnCount int32
	for _, conn := range n.conns {
		idleConnCount = 0
		enableConnCount = 0
		if atomic.LoadInt32(&(conn.state)) == Up {
			idleConnCount = conn.IdleConnCount()
			enableConnCount = conn.EnableConnCount()
		}

		info := NodeConnsInfo{
			State:           conn.State(),
			LastPing:        conn.GetLastPing(),
			Addr:            conn.Addr(),
			IdleConnCount:   idleConnCount,
			EnableConnCount: enableConnCount,
		}
		connsInfos = append(connsInfos, info)
	}
	return connsInfos
}
