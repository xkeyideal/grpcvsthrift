package server

import (
	"fmt"
	"os"

	"grpcvsthrift/rocksdbstore"
	"grpcvsthrift/thrift/rpc"

	"github.com/apache/thrift/lib/go/thrift"
)

type ThriftSever struct {
	port         int
	server       *thrift.TSimpleServer
	rocksdbStore *rocksdbstore.RocksdbStore
	kv           *kvImpl
}

func NewThriftServer(port int, rocksDBDir string) *ThriftSever {
	store, err := rocksdbstore.NewRocksdbStore(rocksDBDir)

	if err != nil {
		panic(err)
	}

	transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()

	serverTransport, err := thrift.NewTServerSocket(fmt.Sprintf("0.0.0.0:%d", port))

	if err != nil {
		fmt.Println("Thrift Engine Err:", err.Error())
		os.Exit(1)
	}

	impl := &kvImpl{
		rocksdbStore: store,
	}

	processor := rpc.NewKVProcessor(impl)

	server := thrift.NewTSimpleServer4(processor, serverTransport, transportFactory, protocolFactory)

	return &ThriftSever{
		port:         port,
		server:       server,
		rocksdbStore: store,
	}
}

func (s *ThriftSever) Stop() {
	if s.server != nil {
		s.server.Stop()
	}

	s.rocksdbStore.Close()
}

func (s *ThriftSever) Start() {
	fmt.Printf("kv running port %d\n", s.port)

	err := s.server.Serve()
	if err != nil {
		fmt.Println("Server Run Error: ", err)
		os.Exit(1)
	}
}
