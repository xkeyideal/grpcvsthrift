package main

import (
	"grpcvsthrift/thrift/server"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	rocksdbDir := "/Users/xkey/raftlab/grpcvsthrift/data/thrift/rocksdb"
	server := server.NewThriftServer(50052, rocksdbDir)

	go server.Start()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	<-signals

	server.Stop()
}
