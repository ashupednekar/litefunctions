module github.com/ashupednekar/litefunctions/ingestor

go 1.24.4

require (
	github.com/ashupednekar/litefunctions/common v0.0.0
	github.com/gorilla/websocket v1.5.3
	github.com/nats-io/nats.go v1.43.0
	go-simpler.org/env v0.12.0
	google.golang.org/grpc v1.78.0
)

replace github.com/ashupednekar/litefunctions/common => ../common

require (
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/nats-io/nkeys v0.4.11 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.44.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
