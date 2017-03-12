package meterstorepb

import (
	"github.com/golang/protobuf/proto"
)

//go:generate  protoc --go_out . record.proto

// MarshalBinary implements encoding.BinaryMarshal.
func (r *TimeRecord) MarshalBinary() ([]byte, error) {
	return proto.Marshal(r)
}

// UnmarshalBinary implements encoding.UnmarshalBinary.
func (r *TimeRecord) UnmarshalBinary(data []byte) error {
	return proto.Unmarshal(data, r)
}
