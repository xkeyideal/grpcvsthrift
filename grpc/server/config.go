package server

type ServerConfig struct {
	Port int

	RocksDBDir string

	MaxRequestBytes uint

	ServiceName string
}
