// Code generated by protoc-gen-go.
// source: record.proto
// DO NOT EDIT!

/*
Package meterstorepb is a generated protocol buffer package.

It is generated from these files:
	record.proto

It has these top-level messages:
	TimeRecord
	MeterInfo
	MeterRecord
*/
package meterstorepb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type TimeRecord struct {
	Timestamp    uint64 `protobuf:"varint,1,opt,name=timestamp" json:"timestamp,omitempty"`
	InLog        bool   `protobuf:"varint,2,opt,name=inLog" json:"inLog,omitempty"`
	MeterId      uint32 `protobuf:"varint,3,opt,name=meterId" json:"meterId,omitempty"`
	Readings     uint32 `protobuf:"varint,4,opt,name=readings" json:"readings,omitempty"`
	SystemPower  int32  `protobuf:"varint,5,opt,name=systemPower" json:"systemPower,omitempty"`
	SystemEnergy int32  `protobuf:"varint,6,opt,name=systemEnergy" json:"systemEnergy,omitempty"`
}

func (m *TimeRecord) Reset()                    { *m = TimeRecord{} }
func (m *TimeRecord) String() string            { return proto.CompactTextString(m) }
func (*TimeRecord) ProtoMessage()               {}
func (*TimeRecord) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type MeterInfo struct {
	Addr     string `protobuf:"bytes,1,opt,name=addr" json:"addr,omitempty"`
	Location int32  `protobuf:"varint,2,opt,name=location" json:"location,omitempty"`
}

func (m *MeterInfo) Reset()                    { *m = MeterInfo{} }
func (m *MeterInfo) String() string            { return proto.CompactTextString(m) }
func (*MeterInfo) ProtoMessage()               {}
func (*MeterInfo) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type MeterRecord struct {
	Meters []*MeterInfo `protobuf:"bytes,1,rep,name=meters" json:"meters,omitempty"`
}

func (m *MeterRecord) Reset()                    { *m = MeterRecord{} }
func (m *MeterRecord) String() string            { return proto.CompactTextString(m) }
func (*MeterRecord) ProtoMessage()               {}
func (*MeterRecord) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *MeterRecord) GetMeters() []*MeterInfo {
	if m != nil {
		return m.Meters
	}
	return nil
}

func init() {
	proto.RegisterType((*TimeRecord)(nil), "TimeRecord")
	proto.RegisterType((*MeterInfo)(nil), "MeterInfo")
	proto.RegisterType((*MeterRecord)(nil), "MeterRecord")
}

func init() { proto.RegisterFile("record.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 243 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x54, 0x90, 0xbf, 0x4b, 0xc4, 0x30,
	0x14, 0xc7, 0x89, 0xd7, 0xd6, 0xeb, 0x6b, 0x75, 0x08, 0x0e, 0x41, 0x1c, 0x42, 0xa6, 0x4c, 0x07,
	0xea, 0xe8, 0x26, 0x38, 0x08, 0x0a, 0x12, 0x9c, 0xdc, 0x7a, 0x97, 0x67, 0x09, 0x98, 0xa4, 0x24,
	0x01, 0xb9, 0xff, 0xcd, 0x3f, 0x4e, 0x7c, 0x3d, 0xab, 0xb7, 0xbd, 0xcf, 0xf7, 0x9b, 0x1f, 0x9f,
	0x04, 0xfa, 0x84, 0xbb, 0x98, 0xec, 0x66, 0x4a, 0xb1, 0x44, 0xf5, 0xc5, 0x00, 0x5e, 0x9d, 0x47,
	0x43, 0x21, 0xbf, 0x82, 0xb6, 0x38, 0x8f, 0xb9, 0x0c, 0x7e, 0x12, 0x4c, 0x32, 0x5d, 0x99, 0xbf,
	0x80, 0x5f, 0x40, 0xed, 0xc2, 0x53, 0x1c, 0xc5, 0x89, 0x64, 0x7a, 0x6d, 0x66, 0xe0, 0x02, 0x4e,
	0x3d, 0x16, 0x4c, 0x8f, 0x56, 0xac, 0x24, 0xd3, 0x67, 0xe6, 0x17, 0xf9, 0x25, 0xac, 0x13, 0x0e,
	0xd6, 0x85, 0x31, 0x8b, 0x8a, 0xaa, 0x85, 0xb9, 0x84, 0x2e, 0xef, 0x73, 0x41, 0xff, 0x12, 0x3f,
	0x31, 0x89, 0x5a, 0x32, 0x5d, 0x9b, 0xff, 0x11, 0x57, 0xd0, 0xcf, 0xf8, 0x10, 0x30, 0x8d, 0x7b,
	0xd1, 0xd0, 0x92, 0xa3, 0x4c, 0xdd, 0x41, 0xfb, 0x4c, 0x97, 0x85, 0xf7, 0xc8, 0x39, 0x54, 0x83,
	0xb5, 0x89, 0xbc, 0x5b, 0x43, 0xf3, 0x8f, 0xc2, 0x47, 0xdc, 0x0d, 0xc5, 0xc5, 0x40, 0xd6, 0xb5,
	0x59, 0x58, 0x5d, 0x43, 0x47, 0x9b, 0x0f, 0x6f, 0x57, 0xd0, 0x90, 0x78, 0x16, 0x4c, 0xae, 0x74,
	0x77, 0x03, 0x9b, 0xe5, 0x68, 0x73, 0x68, 0xee, 0xcf, 0xdf, 0xfa, 0x79, 0x2a, 0x31, 0xe1, 0xb4,
	0xdd, 0x36, 0xf4, 0x8b, 0xb7, 0xdf, 0x01, 0x00, 0x00, 0xff, 0xff, 0x60, 0x9d, 0x82, 0xaf, 0x55,
	0x01, 0x00, 0x00,
}
