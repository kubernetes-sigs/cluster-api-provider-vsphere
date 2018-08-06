// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/cloud/dialogflow/v2beta1/session_entity_type.proto

package dialogflow // import "google.golang.org/genproto/googleapis/cloud/dialogflow/v2beta1"

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import empty "github.com/golang/protobuf/ptypes/empty"
import _ "google.golang.org/genproto/googleapis/api/annotations"
import field_mask "google.golang.org/genproto/protobuf/field_mask"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// The types of modifications for a session entity type.
type SessionEntityType_EntityOverrideMode int32

const (
	// Not specified. This value should be never used.
	SessionEntityType_ENTITY_OVERRIDE_MODE_UNSPECIFIED SessionEntityType_EntityOverrideMode = 0
	// The collection of session entities overrides the collection of entities
	// in the corresponding developer entity type.
	SessionEntityType_ENTITY_OVERRIDE_MODE_OVERRIDE SessionEntityType_EntityOverrideMode = 1
	// The collection of session entities extends the collection of entities in
	// the corresponding developer entity type.
	// Calls to `ListSessionEntityTypes`, `GetSessionEntityType`,
	// `CreateSessionEntityType` and `UpdateSessionEntityType` return the full
	// collection of entities from the developer entity type in the agent's
	// default language and the session entity type.
	SessionEntityType_ENTITY_OVERRIDE_MODE_SUPPLEMENT SessionEntityType_EntityOverrideMode = 2
)

var SessionEntityType_EntityOverrideMode_name = map[int32]string{
	0: "ENTITY_OVERRIDE_MODE_UNSPECIFIED",
	1: "ENTITY_OVERRIDE_MODE_OVERRIDE",
	2: "ENTITY_OVERRIDE_MODE_SUPPLEMENT",
}
var SessionEntityType_EntityOverrideMode_value = map[string]int32{
	"ENTITY_OVERRIDE_MODE_UNSPECIFIED": 0,
	"ENTITY_OVERRIDE_MODE_OVERRIDE":    1,
	"ENTITY_OVERRIDE_MODE_SUPPLEMENT":  2,
}

func (x SessionEntityType_EntityOverrideMode) String() string {
	return proto.EnumName(SessionEntityType_EntityOverrideMode_name, int32(x))
}
func (SessionEntityType_EntityOverrideMode) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_session_entity_type_0a5ede95d4809454, []int{0, 0}
}

// Represents a session entity type.
//
// Extends or replaces a developer entity type at the user session level (we
// refer to the entity types defined at the agent level as "developer entity
// types").
//
// Note: session entity types apply to all queries, regardless of the language.
type SessionEntityType struct {
	// Required. The unique identifier of this session entity type. Format:
	// `projects/<Project ID>/agent/sessions/<Session ID>/entityTypes/<Entity Type
	// Display Name>`, or
	// `projects/<Project ID>/agent/environments/<Environment ID>/users/<User
	// ID>/sessions/<Session ID>/entityTypes/<Entity Type Display Name>`.
	// If `Environment ID` is not specified, we assume default 'draft'
	// environment. If `User ID` is not specified, we assume default '-' user.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// Required. Indicates whether the additional data should override or
	// supplement the developer entity type definition.
	EntityOverrideMode SessionEntityType_EntityOverrideMode `protobuf:"varint,2,opt,name=entity_override_mode,json=entityOverrideMode,proto3,enum=google.cloud.dialogflow.v2beta1.SessionEntityType_EntityOverrideMode" json:"entity_override_mode,omitempty"`
	// Required. The collection of entities associated with this session entity
	// type.
	Entities             []*EntityType_Entity `protobuf:"bytes,3,rep,name=entities,proto3" json:"entities,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *SessionEntityType) Reset()         { *m = SessionEntityType{} }
func (m *SessionEntityType) String() string { return proto.CompactTextString(m) }
func (*SessionEntityType) ProtoMessage()    {}
func (*SessionEntityType) Descriptor() ([]byte, []int) {
	return fileDescriptor_session_entity_type_0a5ede95d4809454, []int{0}
}
func (m *SessionEntityType) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SessionEntityType.Unmarshal(m, b)
}
func (m *SessionEntityType) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SessionEntityType.Marshal(b, m, deterministic)
}
func (dst *SessionEntityType) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SessionEntityType.Merge(dst, src)
}
func (m *SessionEntityType) XXX_Size() int {
	return xxx_messageInfo_SessionEntityType.Size(m)
}
func (m *SessionEntityType) XXX_DiscardUnknown() {
	xxx_messageInfo_SessionEntityType.DiscardUnknown(m)
}

var xxx_messageInfo_SessionEntityType proto.InternalMessageInfo

func (m *SessionEntityType) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *SessionEntityType) GetEntityOverrideMode() SessionEntityType_EntityOverrideMode {
	if m != nil {
		return m.EntityOverrideMode
	}
	return SessionEntityType_ENTITY_OVERRIDE_MODE_UNSPECIFIED
}

func (m *SessionEntityType) GetEntities() []*EntityType_Entity {
	if m != nil {
		return m.Entities
	}
	return nil
}

// The request message for [SessionEntityTypes.ListSessionEntityTypes][google.cloud.dialogflow.v2beta1.SessionEntityTypes.ListSessionEntityTypes].
type ListSessionEntityTypesRequest struct {
	// Required. The session to list all session entity types from.
	// Format: `projects/<Project ID>/agent/sessions/<Session ID>` or
	// `projects/<Project ID>/agent/environments/<Environment ID>/users/<User ID>/
	// sessions/<Session ID>`.
	// If `Environment ID` is not specified, we assume default 'draft'
	// environment. If `User ID` is not specified, we assume default '-' user.
	Parent string `protobuf:"bytes,1,opt,name=parent,proto3" json:"parent,omitempty"`
	// Optional. The maximum number of items to return in a single page. By
	// default 100 and at most 1000.
	PageSize int32 `protobuf:"varint,2,opt,name=page_size,json=pageSize,proto3" json:"page_size,omitempty"`
	// Optional. The next_page_token value returned from a previous list request.
	PageToken            string   `protobuf:"bytes,3,opt,name=page_token,json=pageToken,proto3" json:"page_token,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ListSessionEntityTypesRequest) Reset()         { *m = ListSessionEntityTypesRequest{} }
func (m *ListSessionEntityTypesRequest) String() string { return proto.CompactTextString(m) }
func (*ListSessionEntityTypesRequest) ProtoMessage()    {}
func (*ListSessionEntityTypesRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_session_entity_type_0a5ede95d4809454, []int{1}
}
func (m *ListSessionEntityTypesRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ListSessionEntityTypesRequest.Unmarshal(m, b)
}
func (m *ListSessionEntityTypesRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ListSessionEntityTypesRequest.Marshal(b, m, deterministic)
}
func (dst *ListSessionEntityTypesRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListSessionEntityTypesRequest.Merge(dst, src)
}
func (m *ListSessionEntityTypesRequest) XXX_Size() int {
	return xxx_messageInfo_ListSessionEntityTypesRequest.Size(m)
}
func (m *ListSessionEntityTypesRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ListSessionEntityTypesRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ListSessionEntityTypesRequest proto.InternalMessageInfo

func (m *ListSessionEntityTypesRequest) GetParent() string {
	if m != nil {
		return m.Parent
	}
	return ""
}

func (m *ListSessionEntityTypesRequest) GetPageSize() int32 {
	if m != nil {
		return m.PageSize
	}
	return 0
}

func (m *ListSessionEntityTypesRequest) GetPageToken() string {
	if m != nil {
		return m.PageToken
	}
	return ""
}

// The response message for [SessionEntityTypes.ListSessionEntityTypes][google.cloud.dialogflow.v2beta1.SessionEntityTypes.ListSessionEntityTypes].
type ListSessionEntityTypesResponse struct {
	// The list of session entity types. There will be a maximum number of items
	// returned based on the page_size field in the request.
	SessionEntityTypes []*SessionEntityType `protobuf:"bytes,1,rep,name=session_entity_types,json=sessionEntityTypes,proto3" json:"session_entity_types,omitempty"`
	// Token to retrieve the next page of results, or empty if there are no
	// more results in the list.
	NextPageToken        string   `protobuf:"bytes,2,opt,name=next_page_token,json=nextPageToken,proto3" json:"next_page_token,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ListSessionEntityTypesResponse) Reset()         { *m = ListSessionEntityTypesResponse{} }
func (m *ListSessionEntityTypesResponse) String() string { return proto.CompactTextString(m) }
func (*ListSessionEntityTypesResponse) ProtoMessage()    {}
func (*ListSessionEntityTypesResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_session_entity_type_0a5ede95d4809454, []int{2}
}
func (m *ListSessionEntityTypesResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ListSessionEntityTypesResponse.Unmarshal(m, b)
}
func (m *ListSessionEntityTypesResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ListSessionEntityTypesResponse.Marshal(b, m, deterministic)
}
func (dst *ListSessionEntityTypesResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ListSessionEntityTypesResponse.Merge(dst, src)
}
func (m *ListSessionEntityTypesResponse) XXX_Size() int {
	return xxx_messageInfo_ListSessionEntityTypesResponse.Size(m)
}
func (m *ListSessionEntityTypesResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ListSessionEntityTypesResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ListSessionEntityTypesResponse proto.InternalMessageInfo

func (m *ListSessionEntityTypesResponse) GetSessionEntityTypes() []*SessionEntityType {
	if m != nil {
		return m.SessionEntityTypes
	}
	return nil
}

func (m *ListSessionEntityTypesResponse) GetNextPageToken() string {
	if m != nil {
		return m.NextPageToken
	}
	return ""
}

// The request message for [SessionEntityTypes.GetSessionEntityType][google.cloud.dialogflow.v2beta1.SessionEntityTypes.GetSessionEntityType].
type GetSessionEntityTypeRequest struct {
	// Required. The name of the session entity type. Format:
	// `projects/<Project ID>/agent/sessions/<Session ID>/entityTypes/<Entity Type
	// Display Name>` or `projects/<Project ID>/agent/environments/<Environment
	// ID>/users/<User ID>/sessions/<Session ID>/entityTypes/<Entity Type Display
	// Name>`. If `Environment ID` is not specified, we assume default 'draft'
	// environment. If `User ID` is not specified, we assume default '-' user.
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetSessionEntityTypeRequest) Reset()         { *m = GetSessionEntityTypeRequest{} }
func (m *GetSessionEntityTypeRequest) String() string { return proto.CompactTextString(m) }
func (*GetSessionEntityTypeRequest) ProtoMessage()    {}
func (*GetSessionEntityTypeRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_session_entity_type_0a5ede95d4809454, []int{3}
}
func (m *GetSessionEntityTypeRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetSessionEntityTypeRequest.Unmarshal(m, b)
}
func (m *GetSessionEntityTypeRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetSessionEntityTypeRequest.Marshal(b, m, deterministic)
}
func (dst *GetSessionEntityTypeRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetSessionEntityTypeRequest.Merge(dst, src)
}
func (m *GetSessionEntityTypeRequest) XXX_Size() int {
	return xxx_messageInfo_GetSessionEntityTypeRequest.Size(m)
}
func (m *GetSessionEntityTypeRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetSessionEntityTypeRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetSessionEntityTypeRequest proto.InternalMessageInfo

func (m *GetSessionEntityTypeRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

// The request message for [SessionEntityTypes.CreateSessionEntityType][google.cloud.dialogflow.v2beta1.SessionEntityTypes.CreateSessionEntityType].
type CreateSessionEntityTypeRequest struct {
	// Required. The session to create a session entity type for.
	// Format: `projects/<Project ID>/agent/sessions/<Session ID>` or
	// `projects/<Project ID>/agent/environments/<Environment ID>/users/<User ID>/
	// sessions/<Session ID>`. If `Environment ID` is not specified, we assume
	// default 'draft' environment. If `User ID` is not specified, we assume
	// default '-' user.
	Parent string `protobuf:"bytes,1,opt,name=parent,proto3" json:"parent,omitempty"`
	// Required. The session entity type to create.
	SessionEntityType    *SessionEntityType `protobuf:"bytes,2,opt,name=session_entity_type,json=sessionEntityType,proto3" json:"session_entity_type,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *CreateSessionEntityTypeRequest) Reset()         { *m = CreateSessionEntityTypeRequest{} }
func (m *CreateSessionEntityTypeRequest) String() string { return proto.CompactTextString(m) }
func (*CreateSessionEntityTypeRequest) ProtoMessage()    {}
func (*CreateSessionEntityTypeRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_session_entity_type_0a5ede95d4809454, []int{4}
}
func (m *CreateSessionEntityTypeRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CreateSessionEntityTypeRequest.Unmarshal(m, b)
}
func (m *CreateSessionEntityTypeRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CreateSessionEntityTypeRequest.Marshal(b, m, deterministic)
}
func (dst *CreateSessionEntityTypeRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CreateSessionEntityTypeRequest.Merge(dst, src)
}
func (m *CreateSessionEntityTypeRequest) XXX_Size() int {
	return xxx_messageInfo_CreateSessionEntityTypeRequest.Size(m)
}
func (m *CreateSessionEntityTypeRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CreateSessionEntityTypeRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CreateSessionEntityTypeRequest proto.InternalMessageInfo

func (m *CreateSessionEntityTypeRequest) GetParent() string {
	if m != nil {
		return m.Parent
	}
	return ""
}

func (m *CreateSessionEntityTypeRequest) GetSessionEntityType() *SessionEntityType {
	if m != nil {
		return m.SessionEntityType
	}
	return nil
}

// The request message for [SessionEntityTypes.UpdateSessionEntityType][google.cloud.dialogflow.v2beta1.SessionEntityTypes.UpdateSessionEntityType].
type UpdateSessionEntityTypeRequest struct {
	// Required. The entity type to update. Format:
	// `projects/<Project ID>/agent/sessions/<Session ID>/entityTypes/<Entity Type
	// Display Name>` or `projects/<Project ID>/agent/environments/<Environment
	// ID>/users/<User ID>/sessions/<Session ID>/entityTypes/<Entity Type Display
	// Name>`. If `Environment ID` is not specified, we assume default 'draft'
	// environment. If `User ID` is not specified, we assume default '-' user.
	SessionEntityType *SessionEntityType `protobuf:"bytes,1,opt,name=session_entity_type,json=sessionEntityType,proto3" json:"session_entity_type,omitempty"`
	// Optional. The mask to control which fields get updated.
	UpdateMask           *field_mask.FieldMask `protobuf:"bytes,2,opt,name=update_mask,json=updateMask,proto3" json:"update_mask,omitempty"`
	XXX_NoUnkeyedLiteral struct{}              `json:"-"`
	XXX_unrecognized     []byte                `json:"-"`
	XXX_sizecache        int32                 `json:"-"`
}

func (m *UpdateSessionEntityTypeRequest) Reset()         { *m = UpdateSessionEntityTypeRequest{} }
func (m *UpdateSessionEntityTypeRequest) String() string { return proto.CompactTextString(m) }
func (*UpdateSessionEntityTypeRequest) ProtoMessage()    {}
func (*UpdateSessionEntityTypeRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_session_entity_type_0a5ede95d4809454, []int{5}
}
func (m *UpdateSessionEntityTypeRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_UpdateSessionEntityTypeRequest.Unmarshal(m, b)
}
func (m *UpdateSessionEntityTypeRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_UpdateSessionEntityTypeRequest.Marshal(b, m, deterministic)
}
func (dst *UpdateSessionEntityTypeRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_UpdateSessionEntityTypeRequest.Merge(dst, src)
}
func (m *UpdateSessionEntityTypeRequest) XXX_Size() int {
	return xxx_messageInfo_UpdateSessionEntityTypeRequest.Size(m)
}
func (m *UpdateSessionEntityTypeRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_UpdateSessionEntityTypeRequest.DiscardUnknown(m)
}

var xxx_messageInfo_UpdateSessionEntityTypeRequest proto.InternalMessageInfo

func (m *UpdateSessionEntityTypeRequest) GetSessionEntityType() *SessionEntityType {
	if m != nil {
		return m.SessionEntityType
	}
	return nil
}

func (m *UpdateSessionEntityTypeRequest) GetUpdateMask() *field_mask.FieldMask {
	if m != nil {
		return m.UpdateMask
	}
	return nil
}

// The request message for [SessionEntityTypes.DeleteSessionEntityType][google.cloud.dialogflow.v2beta1.SessionEntityTypes.DeleteSessionEntityType].
type DeleteSessionEntityTypeRequest struct {
	// Required. The name of the entity type to delete. Format:
	// `projects/<Project ID>/agent/sessions/<Session ID>/entityTypes/<Entity Type
	// Display Name>` or `projects/<Project ID>/agent/environments/<Environment
	// ID>/users/<User ID>/sessions/<Session ID>/entityTypes/<Entity Type Display
	// Name>`. If `Environment ID` is not specified, we assume default 'draft'
	// environment. If `User ID` is not specified, we assume default '-' user.
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DeleteSessionEntityTypeRequest) Reset()         { *m = DeleteSessionEntityTypeRequest{} }
func (m *DeleteSessionEntityTypeRequest) String() string { return proto.CompactTextString(m) }
func (*DeleteSessionEntityTypeRequest) ProtoMessage()    {}
func (*DeleteSessionEntityTypeRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_session_entity_type_0a5ede95d4809454, []int{6}
}
func (m *DeleteSessionEntityTypeRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DeleteSessionEntityTypeRequest.Unmarshal(m, b)
}
func (m *DeleteSessionEntityTypeRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DeleteSessionEntityTypeRequest.Marshal(b, m, deterministic)
}
func (dst *DeleteSessionEntityTypeRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DeleteSessionEntityTypeRequest.Merge(dst, src)
}
func (m *DeleteSessionEntityTypeRequest) XXX_Size() int {
	return xxx_messageInfo_DeleteSessionEntityTypeRequest.Size(m)
}
func (m *DeleteSessionEntityTypeRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_DeleteSessionEntityTypeRequest.DiscardUnknown(m)
}

var xxx_messageInfo_DeleteSessionEntityTypeRequest proto.InternalMessageInfo

func (m *DeleteSessionEntityTypeRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func init() {
	proto.RegisterType((*SessionEntityType)(nil), "google.cloud.dialogflow.v2beta1.SessionEntityType")
	proto.RegisterType((*ListSessionEntityTypesRequest)(nil), "google.cloud.dialogflow.v2beta1.ListSessionEntityTypesRequest")
	proto.RegisterType((*ListSessionEntityTypesResponse)(nil), "google.cloud.dialogflow.v2beta1.ListSessionEntityTypesResponse")
	proto.RegisterType((*GetSessionEntityTypeRequest)(nil), "google.cloud.dialogflow.v2beta1.GetSessionEntityTypeRequest")
	proto.RegisterType((*CreateSessionEntityTypeRequest)(nil), "google.cloud.dialogflow.v2beta1.CreateSessionEntityTypeRequest")
	proto.RegisterType((*UpdateSessionEntityTypeRequest)(nil), "google.cloud.dialogflow.v2beta1.UpdateSessionEntityTypeRequest")
	proto.RegisterType((*DeleteSessionEntityTypeRequest)(nil), "google.cloud.dialogflow.v2beta1.DeleteSessionEntityTypeRequest")
	proto.RegisterEnum("google.cloud.dialogflow.v2beta1.SessionEntityType_EntityOverrideMode", SessionEntityType_EntityOverrideMode_name, SessionEntityType_EntityOverrideMode_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// SessionEntityTypesClient is the client API for SessionEntityTypes service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type SessionEntityTypesClient interface {
	// Returns the list of all session entity types in the specified session.
	ListSessionEntityTypes(ctx context.Context, in *ListSessionEntityTypesRequest, opts ...grpc.CallOption) (*ListSessionEntityTypesResponse, error)
	// Retrieves the specified session entity type.
	GetSessionEntityType(ctx context.Context, in *GetSessionEntityTypeRequest, opts ...grpc.CallOption) (*SessionEntityType, error)
	// Creates a session entity type.
	CreateSessionEntityType(ctx context.Context, in *CreateSessionEntityTypeRequest, opts ...grpc.CallOption) (*SessionEntityType, error)
	// Updates the specified session entity type.
	UpdateSessionEntityType(ctx context.Context, in *UpdateSessionEntityTypeRequest, opts ...grpc.CallOption) (*SessionEntityType, error)
	// Deletes the specified session entity type.
	DeleteSessionEntityType(ctx context.Context, in *DeleteSessionEntityTypeRequest, opts ...grpc.CallOption) (*empty.Empty, error)
}

type sessionEntityTypesClient struct {
	cc *grpc.ClientConn
}

func NewSessionEntityTypesClient(cc *grpc.ClientConn) SessionEntityTypesClient {
	return &sessionEntityTypesClient{cc}
}

func (c *sessionEntityTypesClient) ListSessionEntityTypes(ctx context.Context, in *ListSessionEntityTypesRequest, opts ...grpc.CallOption) (*ListSessionEntityTypesResponse, error) {
	out := new(ListSessionEntityTypesResponse)
	err := c.cc.Invoke(ctx, "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/ListSessionEntityTypes", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sessionEntityTypesClient) GetSessionEntityType(ctx context.Context, in *GetSessionEntityTypeRequest, opts ...grpc.CallOption) (*SessionEntityType, error) {
	out := new(SessionEntityType)
	err := c.cc.Invoke(ctx, "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/GetSessionEntityType", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sessionEntityTypesClient) CreateSessionEntityType(ctx context.Context, in *CreateSessionEntityTypeRequest, opts ...grpc.CallOption) (*SessionEntityType, error) {
	out := new(SessionEntityType)
	err := c.cc.Invoke(ctx, "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/CreateSessionEntityType", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sessionEntityTypesClient) UpdateSessionEntityType(ctx context.Context, in *UpdateSessionEntityTypeRequest, opts ...grpc.CallOption) (*SessionEntityType, error) {
	out := new(SessionEntityType)
	err := c.cc.Invoke(ctx, "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/UpdateSessionEntityType", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sessionEntityTypesClient) DeleteSessionEntityType(ctx context.Context, in *DeleteSessionEntityTypeRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	out := new(empty.Empty)
	err := c.cc.Invoke(ctx, "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/DeleteSessionEntityType", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SessionEntityTypesServer is the server API for SessionEntityTypes service.
type SessionEntityTypesServer interface {
	// Returns the list of all session entity types in the specified session.
	ListSessionEntityTypes(context.Context, *ListSessionEntityTypesRequest) (*ListSessionEntityTypesResponse, error)
	// Retrieves the specified session entity type.
	GetSessionEntityType(context.Context, *GetSessionEntityTypeRequest) (*SessionEntityType, error)
	// Creates a session entity type.
	CreateSessionEntityType(context.Context, *CreateSessionEntityTypeRequest) (*SessionEntityType, error)
	// Updates the specified session entity type.
	UpdateSessionEntityType(context.Context, *UpdateSessionEntityTypeRequest) (*SessionEntityType, error)
	// Deletes the specified session entity type.
	DeleteSessionEntityType(context.Context, *DeleteSessionEntityTypeRequest) (*empty.Empty, error)
}

func RegisterSessionEntityTypesServer(s *grpc.Server, srv SessionEntityTypesServer) {
	s.RegisterService(&_SessionEntityTypes_serviceDesc, srv)
}

func _SessionEntityTypes_ListSessionEntityTypes_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSessionEntityTypesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SessionEntityTypesServer).ListSessionEntityTypes(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/ListSessionEntityTypes",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SessionEntityTypesServer).ListSessionEntityTypes(ctx, req.(*ListSessionEntityTypesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SessionEntityTypes_GetSessionEntityType_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSessionEntityTypeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SessionEntityTypesServer).GetSessionEntityType(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/GetSessionEntityType",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SessionEntityTypesServer).GetSessionEntityType(ctx, req.(*GetSessionEntityTypeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SessionEntityTypes_CreateSessionEntityType_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateSessionEntityTypeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SessionEntityTypesServer).CreateSessionEntityType(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/CreateSessionEntityType",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SessionEntityTypesServer).CreateSessionEntityType(ctx, req.(*CreateSessionEntityTypeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SessionEntityTypes_UpdateSessionEntityType_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateSessionEntityTypeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SessionEntityTypesServer).UpdateSessionEntityType(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/UpdateSessionEntityType",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SessionEntityTypesServer).UpdateSessionEntityType(ctx, req.(*UpdateSessionEntityTypeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SessionEntityTypes_DeleteSessionEntityType_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteSessionEntityTypeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SessionEntityTypesServer).DeleteSessionEntityType(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/google.cloud.dialogflow.v2beta1.SessionEntityTypes/DeleteSessionEntityType",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SessionEntityTypesServer).DeleteSessionEntityType(ctx, req.(*DeleteSessionEntityTypeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _SessionEntityTypes_serviceDesc = grpc.ServiceDesc{
	ServiceName: "google.cloud.dialogflow.v2beta1.SessionEntityTypes",
	HandlerType: (*SessionEntityTypesServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListSessionEntityTypes",
			Handler:    _SessionEntityTypes_ListSessionEntityTypes_Handler,
		},
		{
			MethodName: "GetSessionEntityType",
			Handler:    _SessionEntityTypes_GetSessionEntityType_Handler,
		},
		{
			MethodName: "CreateSessionEntityType",
			Handler:    _SessionEntityTypes_CreateSessionEntityType_Handler,
		},
		{
			MethodName: "UpdateSessionEntityType",
			Handler:    _SessionEntityTypes_UpdateSessionEntityType_Handler,
		},
		{
			MethodName: "DeleteSessionEntityType",
			Handler:    _SessionEntityTypes_DeleteSessionEntityType_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "google/cloud/dialogflow/v2beta1/session_entity_type.proto",
}

func init() {
	proto.RegisterFile("google/cloud/dialogflow/v2beta1/session_entity_type.proto", fileDescriptor_session_entity_type_0a5ede95d4809454)
}

var fileDescriptor_session_entity_type_0a5ede95d4809454 = []byte{
	// 870 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x56, 0xcf, 0x6f, 0xe3, 0x44,
	0x14, 0x66, 0x5c, 0x58, 0xed, 0xce, 0xf2, 0xa3, 0x3b, 0x54, 0x69, 0x94, 0xd2, 0x34, 0x78, 0x11,
	0xaa, 0x72, 0xb0, 0xd5, 0xc0, 0x65, 0x59, 0x7e, 0x48, 0xdb, 0xb8, 0xab, 0x48, 0x9b, 0x34, 0x72,
	0xd2, 0x4a, 0xf4, 0x62, 0x39, 0xf5, 0xab, 0x65, 0x9a, 0xcc, 0x18, 0xcf, 0xa4, 0x25, 0xad, 0x7a,
	0xe9, 0x95, 0x03, 0x07, 0x24, 0x4e, 0x5c, 0xe0, 0xc8, 0x81, 0x03, 0xe2, 0xc2, 0x0d, 0xce, 0x9c,
	0x10, 0x7f, 0x01, 0x12, 0x07, 0xfe, 0x01, 0x24, 0xb8, 0x21, 0x8f, 0x9d, 0xa6, 0xd4, 0xbf, 0xda,
	0x6c, 0x4f, 0xb1, 0x9f, 0xe7, 0x7b, 0xef, 0x7d, 0x9f, 0xdf, 0xfb, 0x62, 0xfc, 0xc8, 0x65, 0xcc,
	0x1d, 0x82, 0xbe, 0x3f, 0x64, 0x63, 0x47, 0x77, 0x3c, 0x7b, 0xc8, 0xdc, 0x83, 0x21, 0x3b, 0xd6,
	0x8f, 0x1a, 0x03, 0x10, 0xf6, 0x86, 0xce, 0x81, 0x73, 0x8f, 0x51, 0x0b, 0xa8, 0xf0, 0xc4, 0xc4,
	0x12, 0x13, 0x1f, 0x34, 0x3f, 0x60, 0x82, 0x91, 0xb5, 0x08, 0xaa, 0x49, 0xa8, 0x36, 0x83, 0x6a,
	0x31, 0xb4, 0xf2, 0x46, 0x9c, 0xdb, 0xf6, 0x3d, 0xdd, 0xa6, 0x94, 0x09, 0x5b, 0x78, 0x8c, 0xf2,
	0x08, 0x5e, 0xd9, 0x28, 0xaa, 0x9c, 0xa8, 0x58, 0x59, 0x89, 0x21, 0xf2, 0x6e, 0x30, 0x3e, 0xd0,
	0x61, 0xe4, 0x8b, 0x49, 0xfc, 0xb0, 0x76, 0xf5, 0xe1, 0x81, 0x07, 0x43, 0xc7, 0x1a, 0xd9, 0xfc,
	0x30, 0x3a, 0xa1, 0xfe, 0xad, 0xe0, 0x07, 0xbd, 0x88, 0x8e, 0x21, 0x73, 0xf7, 0x27, 0x3e, 0x10,
	0x82, 0x5f, 0xa4, 0xf6, 0x08, 0xca, 0xa8, 0x86, 0xd6, 0xef, 0x99, 0xf2, 0x9a, 0x1c, 0xe3, 0xa5,
	0xb8, 0x3a, 0x3b, 0x82, 0x20, 0xf0, 0x1c, 0xb0, 0x46, 0xcc, 0x81, 0xb2, 0x52, 0x43, 0xeb, 0xaf,
	0x36, 0x0c, 0xad, 0x80, 0xb9, 0x96, 0xa8, 0xa2, 0x45, 0x97, 0xdb, 0x71, 0xb6, 0x36, 0x73, 0xc0,
	0x24, 0x90, 0x88, 0x91, 0x0e, 0xbe, 0x2b, 0xa3, 0x1e, 0xf0, 0xf2, 0x42, 0x6d, 0x61, 0xfd, 0x7e,
	0xa3, 0x51, 0x58, 0x2c, 0x51, 0xc5, 0xbc, 0xc8, 0xa1, 0x9e, 0x23, 0x4c, 0x92, 0xa5, 0xc9, 0x5b,
	0xb8, 0x66, 0x74, 0xfa, 0xad, 0xfe, 0xc7, 0xd6, 0xf6, 0xae, 0x61, 0x9a, 0xad, 0xa6, 0x61, 0xb5,
	0xb7, 0x9b, 0x86, 0xb5, 0xd3, 0xe9, 0x75, 0x8d, 0xcd, 0xd6, 0x56, 0xcb, 0x68, 0x2e, 0xbe, 0x40,
	0xde, 0xc4, 0xab, 0xa9, 0xa7, 0xa6, 0x77, 0x8b, 0x88, 0x3c, 0xc4, 0x6b, 0xa9, 0x47, 0x7a, 0x3b,
	0xdd, 0xee, 0x33, 0xa3, 0x6d, 0x74, 0xfa, 0x8b, 0x8a, 0xca, 0xf1, 0xea, 0x33, 0x8f, 0x8b, 0x84,
	0x28, 0xdc, 0x84, 0x4f, 0xc7, 0xc0, 0x05, 0x29, 0xe1, 0x3b, 0xbe, 0x1d, 0x00, 0x15, 0xf1, 0x4b,
	0x88, 0xef, 0xc8, 0x0a, 0xbe, 0xe7, 0xdb, 0x2e, 0x58, 0xdc, 0x3b, 0x89, 0xb4, 0x7f, 0xc9, 0xbc,
	0x1b, 0x06, 0x7a, 0xde, 0x09, 0x90, 0x55, 0x8c, 0xe5, 0x43, 0xc1, 0x0e, 0x81, 0x96, 0x17, 0x24,
	0x50, 0x1e, 0xef, 0x87, 0x01, 0xf5, 0x7b, 0x84, 0xab, 0x59, 0x55, 0xb9, 0xcf, 0x28, 0x07, 0xe2,
	0xe0, 0xa5, 0x94, 0xe9, 0xe6, 0x65, 0x74, 0x4d, 0xe1, 0x13, 0xa9, 0x4d, 0xc2, 0x13, 0xd5, 0xc8,
	0xdb, 0xf8, 0x35, 0x0a, 0x9f, 0x09, 0xeb, 0x52, 0xb3, 0x8a, 0x6c, 0xf6, 0x95, 0x30, 0xdc, 0xbd,
	0x68, 0x78, 0x03, 0xaf, 0x3c, 0x85, 0x64, 0xbb, 0x53, 0x8d, 0x52, 0xc6, 0x54, 0xfd, 0x1a, 0xe1,
	0xea, 0x66, 0x00, 0xb6, 0x80, 0x4c, 0x58, 0x96, 0xb4, 0x03, 0xfc, 0x7a, 0x0a, 0x77, 0xd9, 0xd9,
	0x7c, 0xd4, 0x1f, 0x24, 0xa8, 0xab, 0xbf, 0x20, 0x5c, 0xdd, 0xf1, 0x9d, 0xbc, 0xf6, 0x32, 0xda,
	0x40, 0xb7, 0xd8, 0x06, 0x79, 0x8c, 0xef, 0x8f, 0x65, 0x17, 0xd2, 0x0b, 0x62, 0x8a, 0x95, 0x69,
	0xee, 0xa9, 0x5d, 0x68, 0x5b, 0xa1, 0x5d, 0xb4, 0x6d, 0x7e, 0x68, 0xe2, 0xe8, 0x78, 0x78, 0xad,
	0xbe, 0x8b, 0xab, 0x4d, 0x18, 0x42, 0x0e, 0x85, 0x94, 0x17, 0xd3, 0xf8, 0xf5, 0x65, 0x4c, 0x92,
	0x83, 0x47, 0x7e, 0x50, 0x70, 0x29, 0x7d, 0x26, 0xc9, 0x87, 0x85, 0x5c, 0x73, 0x57, 0xa8, 0xf2,
	0xd1, 0xdc, 0xf8, 0x68, 0x19, 0xd4, 0xaf, 0xd0, 0xf9, 0xef, 0x7f, 0x7e, 0xa9, 0x7c, 0x81, 0xc8,
	0xa3, 0x0b, 0x07, 0x3e, 0x8d, 0x86, 0xe5, 0x03, 0x3f, 0x60, 0x9f, 0xc0, 0xbe, 0xe0, 0x7a, 0x5d,
	0xb7, 0x5d, 0xa0, 0x62, 0xfa, 0xa7, 0xc0, 0xf5, 0xfa, 0x59, 0x6c, 0xd3, 0x32, 0xd9, 0x9e, 0x49,
	0xba, 0xc5, 0x60, 0xa0, 0x47, 0x5e, 0xc0, 0xe8, 0x08, 0xa8, 0x0c, 0x8e, 0x39, 0x04, 0xe1, 0x6f,
	0x46, 0x4e, 0xf2, 0x8d, 0x82, 0x97, 0xd2, 0x16, 0x83, 0xbc, 0x5f, 0x48, 0x39, 0x67, 0x9f, 0x2a,
	0x73, 0x0c, 0x57, 0xba, 0x46, 0xe1, 0x0b, 0xcf, 0x53, 0xe8, 0x32, 0x19, 0xbd, 0x7e, 0xf6, 0x7f,
	0x8d, 0xd2, 0xc1, 0x85, 0x0a, 0x5d, 0xc9, 0x49, 0x7e, 0x53, 0xf0, 0x72, 0x86, 0x11, 0x90, 0xe2,
	0xc9, 0xc8, 0xb7, 0x90, 0xb9, 0x94, 0xfa, 0x39, 0x52, 0xea, 0x27, 0xa4, 0xce, 0x3f, 0x4d, 0xef,
	0xa5, 0x59, 0xc3, 0x9e, 0xab, 0xde, 0xfa, 0x88, 0xa5, 0x16, 0x22, 0xff, 0x2a, 0x78, 0x39, 0xc3,
	0xbd, 0xae, 0xa1, 0x69, 0xbe, 0xef, 0xcd, 0xa5, 0xe9, 0x5f, 0x91, 0xa6, 0x7f, 0xa0, 0x46, 0x7b,
	0xa6, 0x40, 0xda, 0xe7, 0xd9, 0x0d, 0x27, 0x32, 0x5d, 0xe7, 0xd3, 0x86, 0x33, 0x4f, 0x95, 0x9b,
	0x8e, 0x6e, 0xba, 0xf6, 0x9f, 0x2b, 0x78, 0x39, 0xc3, 0x76, 0xaf, 0xa1, 0x7d, 0xbe, 0x61, 0x57,
	0x4a, 0x09, 0xeb, 0x37, 0xc2, 0xcf, 0xc8, 0xd9, 0x76, 0xd7, 0x9f, 0x67, 0xbb, 0xeb, 0xb7, 0xbe,
	0xdd, 0x4f, 0x7e, 0x44, 0xf8, 0xe1, 0x3e, 0x1b, 0x15, 0xf1, 0x7e, 0x52, 0x4a, 0x50, 0xee, 0x86,
	0x0c, 0xbb, 0x68, 0xaf, 0x15, 0x43, 0x5d, 0x36, 0xb4, 0xa9, 0xab, 0xb1, 0xc0, 0xd5, 0x5d, 0xa0,
	0x92, 0xbf, 0x1e, 0x3d, 0xb2, 0x7d, 0x8f, 0x67, 0x7e, 0x8a, 0x3f, 0x9e, 0x85, 0xfe, 0x41, 0xe8,
	0x5b, 0x45, 0x69, 0x6e, 0x7d, 0xa7, 0xac, 0x3d, 0x8d, 0x72, 0x6e, 0xca, 0x76, 0x9a, 0xb3, 0x76,
	0x76, 0x23, 0xd0, 0xe0, 0x8e, 0xcc, 0xff, 0xce, 0x7f, 0x01, 0x00, 0x00, 0xff, 0xff, 0xdc, 0xc9,
	0xd4, 0x00, 0x63, 0x0c, 0x00, 0x00,
}
