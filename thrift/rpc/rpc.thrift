namespace go rpc

//thrift -r -out ../  --gen go rpc.thrift

struct Health {
    1: bool OK,
    2: string Errors
}

struct GetRequest {
    1: string key,
    2: bool serializable
}

struct GetResponse {
    1: ResponseHeader header,
    2: list<KeyValue> kvs
}

struct KeyValue {
    1: string key,
    2: string value
}

struct PutRequest {
    1: string key,
    2: string value
}

struct ResponseHeader {
    1: i64 cluster_id,
    2: i64 node_id,
    3: i64 consume_time,
    4: string node_ip
}

struct PutResponse {
    1: ResponseHeader header
}

service KV {
    Health Ping();
    GetResponse Get(1: GetRequest req);
    PutResponse Put(1: PutRequest req);
    PutResponse MultiPut(1: list<PutRequest> reqs);
}