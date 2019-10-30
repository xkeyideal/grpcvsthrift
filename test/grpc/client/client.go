package main

import (
	"context"
	"fmt"
	"grpcvsthrift/grpc/client"
	pb "grpcvsthrift/grpc/rpcpb"
	"grpcvsthrift/internal/generate"
	"grpcvsthrift/internal/messagequeue"
	"math/rand"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
)

var histogram metrics.Histogram
var counter metrics.Counter
var streamCounter metrics.Counter
var readErrCounter metrics.Counter

type TestGrpcClient struct {
	kvClient pb.KVClient

	sendChan chan []byte
	recvChan chan []byte

	mq *messagequeue.EntryQueue

	exitchan chan struct{}
	wg       sync.WaitGroup

	rand *rand.Rand
	l    *sync.Mutex

	kvSize  int
	rwRatio int // 读写比例
}

func NewTestGrpcClient(size, rwRatio int, kvClient pb.KVClient) *TestGrpcClient {
	tc := &TestGrpcClient{
		kvClient: kvClient,
		sendChan: make(chan []byte, 1000),
		recvChan: make(chan []byte, 1000),

		mq: messagequeue.NewEntryQueue(1024 * 2),

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

//[from,to)
func (tc *TestGrpcClient) rnd(from, to int) int {
	tc.l.Lock()
	n := tc.rand.Intn(to-from) + from
	tc.l.Unlock()
	return n
}

func (tc *TestGrpcClient) consumeMQ() {
	ticker := time.NewTicker(200 * time.Millisecond)
	tc.wg.Add(1)

	for {
		select {
		case <-tc.exitchan:
			goto exit
		case <-ticker.C:
			stream, err := tc.kvClient.PutStream(context.Background())
			if err != nil {
				fmt.Println("1000", err)
				counter.Inc(1)
				continue
			}

			reqs := tc.mq.Get()
			streamCounter.Inc(int64(len(reqs)))

			st := time.Now().UnixNano()
			for _, req := range reqs {
				err = stream.Send(req)
				if err != nil {
					fmt.Println("2000", err)
					counter.Inc(1)
				}
			}

			_, err = stream.CloseAndRecv()
			if err != nil {
				fmt.Println("3000", err)
				counter.Inc(1)
			}
			x := time.Now().UnixNano() - st
			histogram.Update(x)

			//fmt.Println("xxxx", len(reqs))
		}
	}
exit:
	tc.wg.Done()
}

func (tc *TestGrpcClient) genData() {
	tc.wg.Add(1)
	for {
		select {
		case <-tc.exitchan:
			goto exit
		default:
			n := tc.rnd(0, 100)
			kv := generate.GenRandomBytes(tc.kvSize)
			if n < tc.rwRatio {
				tc.recvChan <- kv
			} else {
				tc.sendChan <- kv
			}
		}
	}
exit:
	tc.wg.Done()
}

func (tc *TestGrpcClient) Send() {
	tc.wg.Add(1)
	for {
		select {
		case <-tc.exitchan:
			goto exit
		case kv := <-tc.sendChan:
			// st := time.Now().UnixNano()
			// _, err := tc.kvClient.Put(context.Background(), &pb.PutRequest{Key: kv, Value: kv})
			// if err != nil {
			// 	counter.Inc(1)
			// }

			// x := time.Now().UnixNano() - st
			// histogram.Update(x)

			req := &pb.PutRequest{Key: kv, Value: kv}
			full, _ := tc.mq.Add(req)
			if !full {
				go func() {
					stream, err := tc.kvClient.PutStream(context.Background())
					if err != nil {
						fmt.Println("000", err)
						counter.Inc(1)
						return
					}

					reqs := tc.mq.Get()
					streamCounter.Inc(int64(len(reqs)))
					//fmt.Println("full", len(reqs))
					st := time.Now().UnixNano()
					for _, req := range reqs {
						err = stream.Send(req)
						if err != nil {
							fmt.Println("111", err)
							counter.Inc(1)
						}
					}

					_, err = stream.CloseAndRecv()
					if err != nil {
						fmt.Println("222", err)
						counter.Inc(1)
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

func (tc *TestGrpcClient) Recv() {
	tc.wg.Add(1)
	for {
		select {
		case <-tc.exitchan:
			goto exit
		case kv := <-tc.recvChan:
			st := time.Now().UnixNano()
			_, err := tc.kvClient.Get(context.Background(), &pb.GetRequest{Key: kv})
			if err != nil {
				readErrCounter.Inc(1)
			}

			x := time.Now().UnixNano() - st
			histogram.Update(x)
		}
	}
exit:
	tc.wg.Done()
}

func (tc *TestGrpcClient) Stop() {
	close(tc.exitchan)
	tc.wg.Wait()
}

func main() {
	s := metrics.NewExpDecaySample(10240, 0.015) // or metrics.NewUniformSample(1028)
	histogram = metrics.NewHistogram(s)

	counter = metrics.NewCounter()
	readErrCounter = metrics.NewCounter()
	streamCounter = metrics.NewCounter()

	cfg := &client.Config{
		Endpoints:            []string{"localhost:50051"},
		DialKeepAliveTime:    5 * time.Second,
		DialKeepAliveTimeout: 1 * time.Second,
		PermitWithoutStream:  true,
	}

	client, err := client.NewGRPCClient(cfg)
	if err != nil {
		panic(err)
	}

	defer client.Close()

	c := pb.NewKVClient(client.Conn)

	tc := NewTestGrpcClient(16, 0, c)
	routineNum := 20
	for i := 0; i < routineNum; i++ {
		go tc.Send()
		go tc.Recv()
	}

	time.Sleep(10 * time.Second)

	tc.Stop()

	fmt.Printf("总次数: %d, 写错误数: %d, 读错误数: %d, 线程数: %d, 每个线程连接数: %d, 请求次数: %d\n", streamCounter.Count()/10, counter.Count(), readErrCounter.Count(), 1, 1, 1)
	fmt.Printf("最小值: %dus, 最大值: %dus, 中间值: %.1fus\n", histogram.Min()/1e3, histogram.Max()/1e3, histogram.Mean()/1e3)
	fmt.Printf("75百分位: %.1fus, 90百分位: %.1fus, 95百分位: %.1fus, 99百分位: %.1fus\n", histogram.Percentile(0.75)/1e3, histogram.Percentile(0.9)/1e3, histogram.Percentile(0.95)/1e3, histogram.Percentile(0.99)/1e3)
}
