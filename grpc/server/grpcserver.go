package server

import (
	"fmt"
	pb "grpcvsthrift/grpc/rpcpb"
	"grpcvsthrift/grpc/server/api"
	"grpcvsthrift/rocksdbstore"
	"math"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

const (
	grpcOverheadBytes = 512 * 1024
	maxStreams        = math.MaxUint32
	maxSendBytes      = math.MaxInt32
)

var kaep = keepalive.EnforcementPolicy{
	MinTime:             5 * time.Second, // If a client pings more than once every 5 seconds, terminate the connection
	PermitWithoutStream: true,            // Allow pings even when there are no active streams
}

var kasp = keepalive.ServerParameters{
	MaxConnectionIdle:     15 * time.Second, // If a client is idle for 15 seconds, send a GOAWAY
	MaxConnectionAge:      30 * time.Second, // If any connection is alive for more than 30 seconds, send a GOAWAY
	MaxConnectionAgeGrace: 5 * time.Second,  // Allow 5 seconds for pending RPCs to complete before forcibly closing connections
	Time:                  5 * time.Second,  // Ping the client if it is idle for 5 seconds to ensure the connection is still active
	Timeout:               1 * time.Second,  // Wait 1 second for the ping ack before assuming the connection is dead
}

type GrpcServer struct {
	cfg *ServerConfig

	health *api.HealthServer

	rocksdbStore *rocksdbstore.RocksdbStore

	listener net.Listener
}

func NewGrpcServer(config *ServerConfig) *GrpcServer {
	store, err := rocksdbstore.NewRocksdbStore(config.RocksDBDir)

	if err != nil {
		panic(err)
	}

	return &GrpcServer{
		cfg:          config,
		health:       api.NewHealthServer(),
		rocksdbStore: store,
	}
}

func (server *GrpcServer) Start() {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", server.cfg.Port))
	if err != nil {
		panic(err)
	}

	server.listener = listener

	var opts []grpc.ServerOption

	opts = append(opts, grpc.KeepaliveEnforcementPolicy(kaep), grpc.KeepaliveParams(kasp))

	opts = append(opts, grpc.MaxRecvMsgSize(int(server.cfg.MaxRequestBytes+grpcOverheadBytes)))
	opts = append(opts, grpc.MaxSendMsgSize(maxSendBytes))
	opts = append(opts, grpc.MaxConcurrentStreams(maxStreams))

	grpcServer := grpc.NewServer(opts...)

	pb.RegisterKVServer(grpcServer, server)

	server.health.SetServingStatus(server.cfg.ServiceName, pb.HealthCheckResponse_SERVING)
	pb.RegisterHealthServer(grpcServer, server.health)

	go func() {
		i := 0
		for {

			x := i % 3
			status := pb.HealthCheckResponse_SERVING
			if x == 0 {
				status = pb.HealthCheckResponse_SERVING
			} else if x == 1 {
				status = pb.HealthCheckResponse_NOT_SERVING
			} else if x == 2 {
				status = pb.HealthCheckResponse_UNKNOWN
			}

			server.health.SetServingStatus(server.cfg.ServiceName, status)

			i++
			time.Sleep(2 * time.Second)
		}
	}()

	grpcServer.Serve(listener)
}

func (server *GrpcServer) Stop() {
	server.listener.Close()
	server.rocksdbStore.Close()
}
