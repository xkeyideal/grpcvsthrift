package server

import (
	"context"
	"fmt"
	"grpcvsthrift/rocksdbstore"
	"grpcvsthrift/thrift/rpc"
	"time"

	"github.com/tecbot/gorocksdb"
)

type kvImpl struct {
	rocksdbStore *rocksdbstore.RocksdbStore
}

func (kv *kvImpl) Ping(ctx context.Context) (*rpc.Health, error) {
	return &rpc.Health{
		OK:     true,
		Errors: "",
	}, nil
}

func (kv *kvImpl) Get(ctx context.Context, req *rpc.GetRequest) (*rpc.GetResponse, error) {
	st := time.Now()
	val, err := kv.rocksdbStore.Lookup([]byte(req.Key))
	if err != nil {
		return nil, err
	}

	resp := &rpc.GetResponse{
		Header: &rpc.ResponseHeader{
			ConsumeTime: int64(time.Now().Sub(st)),
			NodeIP:      fmt.Sprintf("0.0.0.0:xxx"),
		},
		Kvs: []*rpc.KeyValue{},
	}

	resp.Kvs = append(resp.Kvs, &rpc.KeyValue{
		Key:   req.Key,
		Value: string(val),
	})

	return resp, nil
}

func (kv *kvImpl) Put(ctx context.Context, req *rpc.PutRequest) (*rpc.PutResponse, error) {
	st := time.Now()
	err := kv.rocksdbStore.Write([]byte(req.Key), []byte(req.Value))
	if err != nil {
		return nil, err
	}

	resp := &rpc.PutResponse{
		Header: &rpc.ResponseHeader{
			ConsumeTime: int64(time.Now().Sub(st)),
			NodeIP:      fmt.Sprintf("0.0.0.0:xxx"),
		},
	}

	return resp, nil
}

func (kv *kvImpl) MultiPut(ctx context.Context, reqs []*rpc.PutRequest) (*rpc.PutResponse, error) {
	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()

	st := time.Now()

	for _, req := range reqs {
		wb.Put([]byte(req.Key), []byte(req.Value))
	}

	err := kv.rocksdbStore.BatchWrite(wb)
	if err != nil {
		return nil, err
	}

	resp := &rpc.PutResponse{
		Header: &rpc.ResponseHeader{
			ConsumeTime: int64(time.Now().Sub(st)),
			NodeIP:      fmt.Sprintf("0.0.0.0:xxx"),
		},
	}

	return resp, nil
}
