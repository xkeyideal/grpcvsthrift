## GRPC vs Thrift

本项目主要用于压测GRPC与Thrift的性能, 下述的压测均是指写性能，未压测读性能和读写混合性能， 底层存储使用rocksdb。

压测的数据量：key, value均是16个字节

### GRPC 压测结果

GPRC使用rpc的性能在2.4W QPS，使用stream的性能在35W QPS

使用gogoprotobuf的rpc性能在2.5W QPS, stream性能在46W QPS

gogoprotobuf 使用的是--gofast_out进行代码生成

GRPC-balancer直接采用的Etcd相关代码

### Thrift 压测结果

thrift使用单条写的性能在1.3W QPS, 批量(2048条)写的性能在60W QPS, 批量(100条)写性能在22W QPS

由于thrift没有类似grpc rpc stream的方式，所以性能的提升依赖批量写的条数

thrift的测试连接全部使用的是连接池模式

### 测试代码介绍

1. grpc 目录均是GRPC协议相关的代码，grpc/rpcpb目录下是协议生成代码
2. thrift目录均是Thrift协议相关的代码，thrift/rpc目录下是协议的生成代码
3. test/grpc是GRPC的压测代码, 分别是client.go 和 server.go
4. test/thrift是Thrift的压测代码, 分别是client.go 和 server.go
5. rocksdbstore目录是RocksDB的存储代码，RocksDB的调参优化全部在store.go文件中
6. grpc使用的是v1.24.0版本, thrift使用的是v0.12.0版本, RocksDB使用的是v6.1.2版本

### Reference 

* [https://github.com/etcd-io/etcd/clientv3/balancer](https://github.com/etcd-io/etcd)
* [https://github.com/grpc/grpc-go](https://github.com/grpc/grpc-go)
* [https://github.com/gogo/protobuf](https://github.com/gogo/protobuf)
* [https://github.com/gogo/grpc-example](https://github.com/gogo/grpc-example)
* [https://github.com/golang/protobuf](https://github.com/golang/protobuf)
* [https://github.com/apache/thrift](https://github.com/apache/thrift)