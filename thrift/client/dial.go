package client

import (
	"errors"
	"grpcvsthrift/thrift/client/pool"
	"grpcvsthrift/thrift/rpc"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
)

func thriftDialFunc(addr string, connTimeout time.Duration) (*pool.IdleClient, error) {
	socket, err := thrift.NewTSocketTimeout(addr, connTimeout)
	if err != nil {
		return nil, err
	}
	transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
	transport, err := transportFactory.GetTransport(socket)
	if err != nil {
		return nil, err
	}

	if err := transport.Open(); err != nil {
		return nil, err
	}

	iprot := protocolFactory.GetProtocol(transport)
	oprot := protocolFactory.GetProtocol(transport)

	client := rpc.NewKVClient(thrift.NewTStandardClient(iprot, oprot))
	return &pool.IdleClient{
		Client:    client,
		Transport: transport,
		Addr:      addr,
	}, nil
}

func thriftClientCloseFunc(c *pool.IdleClient) error {
	if c == nil {
		return errors.New("client is nil")
	}
	err := c.Transport.Close()
	return err
}
