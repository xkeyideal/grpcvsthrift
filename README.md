## GRPC vs Thrift

本项目主要用于压测GRPC与Thrift的性能, 下述的压测均是指写性能，未压测读性能和读写混合性能， 底层存储使用rocksdb。

### GRPC 压测结果

本台式机GRPC压测结果显示：GPRC使用rpc的性能在5W QPS，使用stream的性能在35W QPS

由于为了验证使用gogoprotobuf，导致rpcpb/rpc.proto已经不是原来的生成代码，目前该代码不能运行，后续进行整理

GRPC-balancer直接采用的Etcd相关代码

### Thrift 压测结果

本台式机Thrift压测结果： thrift使用单条写的性能在1.3W QPS, 批量(2048条)写的性能在60W QPS, 批量(100条)写性能在22W QPS

由于thrift没有类似grpc rpc stream的方式，所以性能的提升依赖批量写的条数

### Reference 

* [https://github.com/etcd-io/etcd/clientv3/balancer](https://github.com/etcd-io/etcd)
* [https://github.com/grpc/grpc-go](https://github.com/grpc/grpc-go)
* [https://github.com/gogo/protobuf](https://github.com/gogo/protobuf)
* [https://github.com/gogo/grpc-example](https://github.com/gogo/grpc-example)
* [https://github.com/golang/protobuf](https://github.com/golang/protobuf)
* [https://github.com/apache/thrift](https://github.com/apache/thrift)