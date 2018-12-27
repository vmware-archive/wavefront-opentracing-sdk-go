package wire

//go:generate protoc --gogofaster_out=$GOPATH/src/github.com/wavefronthq/wavefront-opentracing-sdk-go/wire wire.proto

// Run `go get github.com/gogo/protobuf/protoc-gen-gogofaster` to install the
// gogofaster generator binary.
