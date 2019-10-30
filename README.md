## GRPC vs Thrift

本项目主要用于压测GRPC与Thrift的性能, 底层存储使用rocksdb。

本台式机压测结果显示：GPRC使用rpc的性能在5W QPS，使用stream的性能在35W QPS

由于为了验证使用gogoprotobuf，导致rpcpb/rpc.proto已经不是原来的生成代码，目前该代码不能运行，后续进行整理

Thrift的压测性能尚未进行