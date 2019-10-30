package main

import "grpcvsthrift/grpc/server"

//protoc -Irpcpb --go_out=plugins=grpc:rpcpb/ rpcpb/health.proto rpcpb/rpc.proto

// protoc \
//         -I rpcpb \
//         -I $GOPATH/src/ \
//         -I /Users/xkey/software/protoc/include/google/protobuf \
//         --gofast_out=plugins=grpc,\
// Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
// Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
// Mgoogle/protobuf/empty.proto=github.com/gogo/protobuf/types,\
// Mgoogle/api/annotations.proto=github.com/gogo/googleapis/google/api,\
// Mgoogle/protobuf/field_mask.proto=github.com/gogo/protobuf/types:\
// rpcpb/ \
// rpcpb/rpc.proto

//protoc -Irpcpb -I $GOPATH/src/  -I /Users/xkey/software/protoc/include/ --gofast_out=plugins=grpc,Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/field_mask.proto=github.com/gogo/protobuf/types:rpcpb/ rpcpb/rpc.proto

func main() {
	cfg := &server.ServerConfig{
		Port:            50051,
		RocksDBDir:      "/Users/xkey/raftlab/grpcvsthrift/data/grpc/rocksdb",
		MaxRequestBytes: 2 * 1024 * 1024,
		ServiceName:     "grpcvsthrift",
	}

	server := server.NewGrpcServer(cfg)

	server.Start()
}
