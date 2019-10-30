package main

import (
	"context"
	"fmt"
	"grpcvsthrift/internal/generate"
	"grpcvsthrift/internal/messagequeue"
	"grpcvsthrift/thrift/client"
	"grpcvsthrift/thrift/client/pool"
	"grpcvsthrift/thrift/rpc"
	"math/rand"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
)

var histogram metrics.Histogram
var counter metrics.Counter
var streamCounter metrics.Counter
var readErrCounter metrics.Counter

type TestThriftClient struct {
	kvClient *client.Node

	sendChan chan string
	recvChan chan string

	mq *messagequeue.ThriftEntryQueue

	exitchan chan struct{}
	wg       sync.WaitGroup

	rand *rand.Rand
	l    *sync.Mutex

	kvSize  int
	rwRatio int // 读写比例
}

func NewTestThriftClient(size, rwRatio int, kvClient *client.Node) *TestThriftClient {
	tc := &TestThriftClient{
		kvClient: kvClient,
		sendChan: make(chan string, 1000),
		recvChan: make(chan string, 1000),

		mq: messagequeue.NewThriftEntryQueue(100),

		rand: rand.New(rand.NewSource(time.Now().UnixNano() + 9876)),
		l:    &sync.Mutex{},

		kvSize:  size,
		rwRatio: rwRatio,

		exitchan: make(chan struct{}),
		wg:       sync.WaitGroup{},
	}

	go tc.genData()
	go tc.consumeMQ()

	return tc
}

func (tc *TestThriftClient) genData() {
	tc.wg.Add(1)
	for {
		select {
		case <-tc.exitchan:
			goto exit
		default:
			n := tc.rnd(0, 100)
			kv := generate.GenRandomBytes(tc.kvSize)
			if n < tc.rwRatio {
				tc.recvChan <- string(kv)
			} else {
				tc.sendChan <- string(kv)
			}
		}
	}
exit:
	tc.wg.Done()
}

func (tc *TestThriftClient) consumeMQ() {
	conn, err := tc.kvClient.ChooseNodeConn()
	if err != nil {
		panic(err)
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	tc.wg.Add(1)

	for {
		select {
		case <-tc.exitchan:
			goto exit
		case <-ticker.C:
			reqs := tc.mq.Get()
			if len(reqs) <= 0 {
				continue
			}

			streamCounter.Inc(int64(len(reqs)))

			st := time.Now().UnixNano()
			_, err := MultiPut(2*time.Second, reqs, conn)
			if err != nil {
				counter.Inc(1)
				return
			}

			x := time.Now().UnixNano() - st
			histogram.Update(x)
		}
	}
exit:
	tc.wg.Done()
}

func (tc *TestThriftClient) SingleSend() {
	conn, err := tc.kvClient.ChooseNodeConn()
	if err != nil {
		panic(err)
	}

	tc.wg.Add(1)
	for {
		select {
		case <-tc.exitchan:
			goto exit
		case kv := <-tc.sendChan:

			st := time.Now().UnixNano()
			_, err = Put(2*time.Second, &rpc.PutRequest{Key: kv, Value: kv}, conn)
			if err != nil {
				counter.Inc(1)
			}

			x := time.Now().UnixNano() - st
			histogram.Update(x)
		}
	}

exit:
	tc.wg.Done()
}

func (tc *TestThriftClient) MultiSend() {
	conn, err := tc.kvClient.ChooseNodeConn()
	if err != nil {
		panic(err)
	}

	tc.wg.Add(1)
	for {
		select {
		case <-tc.exitchan:
			goto exit
		case kv := <-tc.sendChan:
			req := &rpc.PutRequest{Key: kv, Value: kv}
			full, _ := tc.mq.Add(req)
			if !full {
				go func() {
					reqs := tc.mq.Get()
					if len(reqs) <= 0 {
						return
					}

					streamCounter.Inc(int64(len(reqs)))

					st := time.Now().UnixNano()
					_, err := MultiPut(2*time.Second, reqs, conn)
					if err != nil {
						counter.Inc(1)
						return
					}

					x := time.Now().UnixNano() - st
					histogram.Update(x)
				}()
			}
		}
	}
exit:
	tc.wg.Done()
}

//[from,to)
func (tc *TestThriftClient) rnd(from, to int) int {
	tc.l.Lock()
	n := tc.rand.Intn(to-from) + from
	tc.l.Unlock()
	return n
}

func (tc *TestThriftClient) Stop() {
	close(tc.exitchan)
	tc.wg.Wait()
}

func Get(writeTimeout time.Duration, req *rpc.GetRequest, conn *client.Conn) (*rpc.GetResponse, error) {
	client, err := conn.ThriftPool.Get()
	if err != nil {
		return nil, err
	}

	defer conn.ThriftPool.Put(client)

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	ch := make(chan *rpc.GetResponse, 1)
	ech := make(chan error, 1)
	go func(client *pool.IdleClient, req *rpc.GetRequest) {
		resp, err := client.Client.Get(ctx, req)
		if err != nil {
			ech <- err
		} else {
			ch <- resp
		}
	}(client, req)

	select {
	case rpcResp := <-ch:
		return rpcResp, nil
	case err := <-ech:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func MultiPut(writeTimeout time.Duration, reqs []*rpc.PutRequest, conn *client.Conn) (*rpc.PutResponse, error) {
	client, err := conn.ThriftPool.Get()
	if err != nil {
		return nil, err
	}

	defer conn.ThriftPool.Put(client)

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	ch := make(chan *rpc.PutResponse, 1)
	ech := make(chan error, 1)
	go func(client *pool.IdleClient, reqs []*rpc.PutRequest) {
		resp, err := client.Client.MultiPut(ctx, reqs)
		if err != nil {
			ech <- err
		} else {
			ch <- resp
		}
	}(client, reqs)

	select {
	case rpcResp := <-ch:
		return rpcResp, nil
	case err := <-ech:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func Put(writeTimeout time.Duration, req *rpc.PutRequest, conn *client.Conn) (*rpc.PutResponse, error) {
	client, err := conn.ThriftPool.Get()
	if err != nil {
		return nil, err
	}

	defer conn.ThriftPool.Put(client)

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	ch := make(chan *rpc.PutResponse, 1)
	ech := make(chan error, 1)
	go func(client *pool.IdleClient, req *rpc.PutRequest) {
		resp, err := client.Client.Put(ctx, req)
		if err != nil {
			ech <- err
		} else {
			ch <- resp
		}
	}(client, req)

	select {
	case rpcResp := <-ch:
		return rpcResp, nil
	case err := <-ech:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func main() {
	s := metrics.NewExpDecaySample(10240, 0.015) // or metrics.NewUniformSample(1028)
	histogram = metrics.NewHistogram(s)

	counter = metrics.NewCounter()
	readErrCounter = metrics.NewCounter()
	streamCounter = metrics.NewCounter()

	addrs := []string{"localhost:50052"}
	checkInterval := 3 * time.Second
	connTimeout := 300 * time.Second
	idleTimeout := 120 * time.Second
	readTimeout := 2 * time.Second

	clientNode, err := client.NewServerNode(addrs, checkInterval, connTimeout, idleTimeout, readTimeout)
	if err != nil {
		panic(err)
	}

	tc := NewTestThriftClient(16, 0, clientNode)
	routineNum := 10
	for i := 0; i < routineNum; i++ {
		go tc.MultiSend()
		//go tc.Recv()
	}

	time.Sleep(10 * time.Second)

	tc.Stop()

	// single send
	//fmt.Printf("总次数: %d, 写错误数: %d, 读错误数: %d, 线程数: %d, 每个线程连接数: %d, 请求次数: %d\n", histogram.Count()/10, counter.Count(), readErrCounter.Count(), 1, 1, 1)

	// multi send
	fmt.Printf("总次数: %d, 写错误数: %d, 读错误数: %d, 线程数: %d, 每个线程连接数: %d, 请求次数: %d\n", streamCounter.Count()/10, counter.Count(), readErrCounter.Count(), 1, 1, 1)
	fmt.Printf("最小值: %dus, 最大值: %dus, 中间值: %.1fus\n", histogram.Min()/1e3, histogram.Max()/1e3, histogram.Mean()/1e3)
	fmt.Printf("75百分位: %.1fus, 90百分位: %.1fus, 95百分位: %.1fus, 99百分位: %.1fus\n", histogram.Percentile(0.75)/1e3, histogram.Percentile(0.9)/1e3, histogram.Percentile(0.95)/1e3, histogram.Percentile(0.99)/1e3)

	// conn, err := clientNode.ChooseNodeConn()
	// if err != nil {
	// 	panic(err)
	// }

	// resp, err := Put(2*time.Second, &rpc.PutRequest{Key: "key", Value: "value"}, conn)
	// fmt.Println(resp, err)

	// resp, err := Get(2*time.Second, &rpc.GetRequest{Key: "key"}, conn)
	// fmt.Println(resp, err)
}
