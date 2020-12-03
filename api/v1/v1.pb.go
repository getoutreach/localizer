// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.6.1
// source: v1.proto

package v1

import (
	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type ConsoleLevel int32

const (
	ConsoleLevel_CONSOLE_LEVEL_UNSPECIFIED ConsoleLevel = 0
	ConsoleLevel_CONSOLE_LEVEL_INFO        ConsoleLevel = 1
	ConsoleLevel_CONSOLE_LEVEL_WARN        ConsoleLevel = 2
	ConsoleLevel_CONSOLE_LEVEL_ERROR       ConsoleLevel = 3
)

// Enum value maps for ConsoleLevel.
var (
	ConsoleLevel_name = map[int32]string{
		0: "CONSOLE_LEVEL_UNSPECIFIED",
		1: "CONSOLE_LEVEL_INFO",
		2: "CONSOLE_LEVEL_WARN",
		3: "CONSOLE_LEVEL_ERROR",
	}
	ConsoleLevel_value = map[string]int32{
		"CONSOLE_LEVEL_UNSPECIFIED": 0,
		"CONSOLE_LEVEL_INFO":        1,
		"CONSOLE_LEVEL_WARN":        2,
		"CONSOLE_LEVEL_ERROR":       3,
	}
)

func (x ConsoleLevel) Enum() *ConsoleLevel {
	p := new(ConsoleLevel)
	*p = x
	return p
}

func (x ConsoleLevel) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ConsoleLevel) Descriptor() protoreflect.EnumDescriptor {
	return file_v1_proto_enumTypes[0].Descriptor()
}

func (ConsoleLevel) Type() protoreflect.EnumType {
	return &file_v1_proto_enumTypes[0]
}

func (x ConsoleLevel) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ConsoleLevel.Descriptor instead.
func (ConsoleLevel) EnumDescriptor() ([]byte, []int) {
	return file_v1_proto_rawDescGZIP(), []int{0}
}

type ExposeServiceRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Namespace string   `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
	Service   string   `protobuf:"bytes,2,opt,name=service,proto3" json:"service,omitempty"`
	PortMap   []string `protobuf:"bytes,3,rep,name=port_map,json=portMap,proto3" json:"port_map,omitempty"`
}

func (x *ExposeServiceRequest) Reset() {
	*x = ExposeServiceRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ExposeServiceRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExposeServiceRequest) ProtoMessage() {}

func (x *ExposeServiceRequest) ProtoReflect() protoreflect.Message {
	mi := &file_v1_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExposeServiceRequest.ProtoReflect.Descriptor instead.
func (*ExposeServiceRequest) Descriptor() ([]byte, []int) {
	return file_v1_proto_rawDescGZIP(), []int{0}
}

func (x *ExposeServiceRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *ExposeServiceRequest) GetService() string {
	if x != nil {
		return x.Service
	}
	return ""
}

func (x *ExposeServiceRequest) GetPortMap() []string {
	if x != nil {
		return x.PortMap
	}
	return nil
}

type ListRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ListRequest) Reset() {
	*x = ListRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRequest) ProtoMessage() {}

func (x *ListRequest) ProtoReflect() protoreflect.Message {
	mi := &file_v1_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRequest.ProtoReflect.Descriptor instead.
func (*ListRequest) Descriptor() ([]byte, []int) {
	return file_v1_proto_rawDescGZIP(), []int{1}
}

type StopExposeRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Namespace string `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
	Service   string `protobuf:"bytes,2,opt,name=service,proto3" json:"service,omitempty"`
}

func (x *StopExposeRequest) Reset() {
	*x = StopExposeRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StopExposeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StopExposeRequest) ProtoMessage() {}

func (x *StopExposeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_v1_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StopExposeRequest.ProtoReflect.Descriptor instead.
func (*StopExposeRequest) Descriptor() ([]byte, []int) {
	return file_v1_proto_rawDescGZIP(), []int{2}
}

func (x *StopExposeRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *StopExposeRequest) GetService() string {
	if x != nil {
		return x.Service
	}
	return ""
}

// This will be turned into ConsoleResponse to be generic probably some
// time later in the future.
type ConsoleResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Output level of this console output
	Level ConsoleLevel `protobuf:"varint,1,opt,name=level,proto3,enum=api.v1.ConsoleLevel" json:"level,omitempty"`
	// Message of this console output
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *ConsoleResponse) Reset() {
	*x = ConsoleResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ConsoleResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConsoleResponse) ProtoMessage() {}

func (x *ConsoleResponse) ProtoReflect() protoreflect.Message {
	mi := &file_v1_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConsoleResponse.ProtoReflect.Descriptor instead.
func (*ConsoleResponse) Descriptor() ([]byte, []int) {
	return file_v1_proto_rawDescGZIP(), []int{3}
}

func (x *ConsoleResponse) GetLevel() ConsoleLevel {
	if x != nil {
		return x.Level
	}
	return ConsoleLevel_CONSOLE_LEVEL_UNSPECIFIED
}

func (x *ConsoleResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type ListService struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Namespace    string `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
	Name         string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Status       string `protobuf:"bytes,3,opt,name=status,proto3" json:"status,omitempty"`
	Endpoint     string `protobuf:"bytes,4,opt,name=endpoint,proto3" json:"endpoint,omitempty"`
	StatusReason string `protobuf:"bytes,5,opt,name=status_reason,json=statusReason,proto3" json:"status_reason,omitempty"`
}

func (x *ListService) Reset() {
	*x = ListService{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListService) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListService) ProtoMessage() {}

func (x *ListService) ProtoReflect() protoreflect.Message {
	mi := &file_v1_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListService.ProtoReflect.Descriptor instead.
func (*ListService) Descriptor() ([]byte, []int) {
	return file_v1_proto_rawDescGZIP(), []int{4}
}

func (x *ListService) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *ListService) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ListService) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *ListService) GetEndpoint() string {
	if x != nil {
		return x.Endpoint
	}
	return ""
}

func (x *ListService) GetStatusReason() string {
	if x != nil {
		return x.StatusReason
	}
	return ""
}

type ListResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Services []*ListService `protobuf:"bytes,1,rep,name=services,proto3" json:"services,omitempty"`
}

func (x *ListResponse) Reset() {
	*x = ListResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_v1_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListResponse) ProtoMessage() {}

func (x *ListResponse) ProtoReflect() protoreflect.Message {
	mi := &file_v1_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListResponse.ProtoReflect.Descriptor instead.
func (*ListResponse) Descriptor() ([]byte, []int) {
	return file_v1_proto_rawDescGZIP(), []int{5}
}

func (x *ListResponse) GetServices() []*ListService {
	if x != nil {
		return x.Services
	}
	return nil
}

var File_v1_proto protoreflect.FileDescriptor

var file_v1_proto_rawDesc = []byte{
	0x0a, 0x08, 0x76, 0x31, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x61, 0x70, 0x69, 0x2e,
	0x76, 0x31, 0x22, 0x69, 0x0a, 0x14, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x65, 0x53, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61,
	0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e,
	0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x6d, 0x61, 0x70, 0x18, 0x03,
	0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x70, 0x6f, 0x72, 0x74, 0x4d, 0x61, 0x70, 0x22, 0x0d, 0x0a,
	0x0b, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x4b, 0x0a, 0x11,
	0x53, 0x74, 0x6f, 0x70, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12,
	0x18, 0x0a, 0x07, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x22, 0x57, 0x0a, 0x0f, 0x43, 0x6f, 0x6e,
	0x73, 0x6f, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2a, 0x0a, 0x05,
	0x6c, 0x65, 0x76, 0x65, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x14, 0x2e, 0x61, 0x70,
	0x69, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x4c, 0x65, 0x76, 0x65,
	0x6c, 0x52, 0x05, 0x6c, 0x65, 0x76, 0x65, 0x6c, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x22, 0x98, 0x01, 0x0a, 0x0b, 0x4c, 0x69, 0x73, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x1a, 0x0a, 0x08,
	0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08,
	0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x12, 0x23, 0x0a, 0x0d, 0x73, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x5f, 0x72, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0c, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65, 0x61, 0x73, 0x6f, 0x6e, 0x22, 0x3f, 0x0a,
	0x0c, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2f, 0x0a,
	0x08, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x13, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x53, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x52, 0x08, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x73, 0x2a, 0x76,
	0x0a, 0x0c, 0x43, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x12, 0x1d,
	0x0a, 0x19, 0x43, 0x4f, 0x4e, 0x53, 0x4f, 0x4c, 0x45, 0x5f, 0x4c, 0x45, 0x56, 0x45, 0x4c, 0x5f,
	0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x16, 0x0a,
	0x12, 0x43, 0x4f, 0x4e, 0x53, 0x4f, 0x4c, 0x45, 0x5f, 0x4c, 0x45, 0x56, 0x45, 0x4c, 0x5f, 0x49,
	0x4e, 0x46, 0x4f, 0x10, 0x01, 0x12, 0x16, 0x0a, 0x12, 0x43, 0x4f, 0x4e, 0x53, 0x4f, 0x4c, 0x45,
	0x5f, 0x4c, 0x45, 0x56, 0x45, 0x4c, 0x5f, 0x57, 0x41, 0x52, 0x4e, 0x10, 0x02, 0x12, 0x17, 0x0a,
	0x13, 0x43, 0x4f, 0x4e, 0x53, 0x4f, 0x4c, 0x45, 0x5f, 0x4c, 0x45, 0x56, 0x45, 0x4c, 0x5f, 0x45,
	0x52, 0x52, 0x4f, 0x52, 0x10, 0x03, 0x32, 0xd9, 0x01, 0x0a, 0x10, 0x4c, 0x6f, 0x63, 0x61, 0x6c,
	0x69, 0x7a, 0x65, 0x72, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x4a, 0x0a, 0x0d, 0x45,
	0x78, 0x70, 0x6f, 0x73, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x1c, 0x2e, 0x61,
	0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x65, 0x53, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x30, 0x01, 0x12, 0x44, 0x0a, 0x0a, 0x53, 0x74, 0x6f, 0x70, 0x45,
	0x78, 0x70, 0x6f, 0x73, 0x65, 0x12, 0x19, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x53,
	0x74, 0x6f, 0x70, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x17, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x6f, 0x6c,
	0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x30, 0x01, 0x12, 0x33, 0x0a,
	0x04, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x13, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x76, 0x31, 0x2e, 0x4c,
	0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x76, 0x31, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x00, 0x42, 0x29, 0x5a, 0x27, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x6a, 0x61, 0x72, 0x65, 0x64, 0x61, 0x6c, 0x6c, 0x61, 0x72, 0x64, 0x2f, 0x6c, 0x6f, 0x63,
	0x61, 0x6c, 0x69, 0x7a, 0x65, 0x72, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_v1_proto_rawDescOnce sync.Once
	file_v1_proto_rawDescData = file_v1_proto_rawDesc
)

func file_v1_proto_rawDescGZIP() []byte {
	file_v1_proto_rawDescOnce.Do(func() {
		file_v1_proto_rawDescData = protoimpl.X.CompressGZIP(file_v1_proto_rawDescData)
	})
	return file_v1_proto_rawDescData
}

var file_v1_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_v1_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_v1_proto_goTypes = []interface{}{
	(ConsoleLevel)(0),            // 0: api.v1.ConsoleLevel
	(*ExposeServiceRequest)(nil), // 1: api.v1.ExposeServiceRequest
	(*ListRequest)(nil),          // 2: api.v1.ListRequest
	(*StopExposeRequest)(nil),    // 3: api.v1.StopExposeRequest
	(*ConsoleResponse)(nil),      // 4: api.v1.ConsoleResponse
	(*ListService)(nil),          // 5: api.v1.ListService
	(*ListResponse)(nil),         // 6: api.v1.ListResponse
}
var file_v1_proto_depIdxs = []int32{
	0, // 0: api.v1.ConsoleResponse.level:type_name -> api.v1.ConsoleLevel
	5, // 1: api.v1.ListResponse.services:type_name -> api.v1.ListService
	1, // 2: api.v1.LocalizerService.ExposeService:input_type -> api.v1.ExposeServiceRequest
	3, // 3: api.v1.LocalizerService.StopExpose:input_type -> api.v1.StopExposeRequest
	2, // 4: api.v1.LocalizerService.List:input_type -> api.v1.ListRequest
	4, // 5: api.v1.LocalizerService.ExposeService:output_type -> api.v1.ConsoleResponse
	4, // 6: api.v1.LocalizerService.StopExpose:output_type -> api.v1.ConsoleResponse
	6, // 7: api.v1.LocalizerService.List:output_type -> api.v1.ListResponse
	5, // [5:8] is the sub-list for method output_type
	2, // [2:5] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_v1_proto_init() }
func file_v1_proto_init() {
	if File_v1_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_v1_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ExposeServiceRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_v1_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_v1_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StopExposeRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_v1_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ConsoleResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_v1_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListService); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_v1_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_v1_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_v1_proto_goTypes,
		DependencyIndexes: file_v1_proto_depIdxs,
		EnumInfos:         file_v1_proto_enumTypes,
		MessageInfos:      file_v1_proto_msgTypes,
	}.Build()
	File_v1_proto = out.File
	file_v1_proto_rawDesc = nil
	file_v1_proto_goTypes = nil
	file_v1_proto_depIdxs = nil
}
