package server

import (
	"context"
	"fmt"
	pb "grpcvsthrift/grpc/rpcpb"
	"time"
)

func (server *GrpcServer) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	st := time.Now()
	// val, err := server.rocksdbStore.Lookup(req.Key)
	// if err != nil {
	// 	return nil, err
	// }

	resp := &pb.GetResponse{
		Header: &pb.ResponseHeader{
			ConsumeTime: uint64(time.Now().Sub(st)),
			NodeIp:      fmt.Sprintf("0.0.0.0:%d", server.cfg.Port),
		},
		Kvs: []*pb.KeyValue{},
	}

	resp.Kvs = append(resp.Kvs, &pb.KeyValue{
		Key:   req.Key,
		Value: req.Key,
	})

	return resp, nil
}

func (server *GrpcServer) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	st := time.Now()
	// err := server.rocksdbStore.Write(req.Key, req.Value)
	// if err != nil {
	// 	return nil, err
	// }

	resp := &pb.PutResponse{
		Header: &pb.ResponseHeader{
			ConsumeTime: uint64(time.Now().Sub(st)),
			NodeIp:      fmt.Sprintf("0.0.0.0:%d", server.cfg.Port),
		},
	}

	return resp, nil
}

func (server *GrpcServer) PutStream(stream pb.KV_PutStreamServer) error {
	// wb := gorocksdb.NewWriteBatch()
	// defer wb.Destroy()

	st := time.Now()

	// for {
	// 	req, err := stream.Recv()
	// 	if err == io.EOF {
	// 		break
	// 	}

	// 	if err != nil {
	// 		break
	// 		//return err
	// 	}

	// 	wb.Put(req.Key, req.Value)
	// 	// err = server.rocksdbStore.Write(req.Key, req.Value)
	// 	// if err != nil {
	// 	// 	return err
	// 	// }
	// }

	// err := server.rocksdbStore.BatchWrite(wb)
	// if err != nil {
	// 	return err
	// }

	resp := &pb.PutResponse{
		Header: &pb.ResponseHeader{
			ConsumeTime: uint64(time.Now().Sub(st)),
			NodeIp:      fmt.Sprintf("0.0.0.0:%d", server.cfg.Port),
		},
	}

	return stream.SendAndClose(resp)
}
