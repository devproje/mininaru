// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package mininaruv1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type PairingState int32

const (
	PairingState_PAIRING_STATE_UNSPECIFIED PairingState = 0
	PairingState_PAIRING_STATE_WAITING     PairingState = 1
	PairingState_PAIRING_STATE_APPROVED    PairingState = 2
	PairingState_PAIRING_STATE_DENIED      PairingState = 3
	PairingState_PAIRING_STATE_EXPIRED     PairingState = 4
)

var (
	PairingState_name = map[int32]string{
		0: "PAIRING_STATE_UNSPECIFIED",
		1: "PAIRING_STATE_WAITING",
		2: "PAIRING_STATE_APPROVED",
		3: "PAIRING_STATE_DENIED",
		4: "PAIRING_STATE_EXPIRED",
	}
	PairingState_value = map[string]int32{
		"PAIRING_STATE_UNSPECIFIED": 0,
		"PAIRING_STATE_WAITING":     1,
		"PAIRING_STATE_APPROVED":    2,
		"PAIRING_STATE_DENIED":      3,
		"PAIRING_STATE_EXPIRED":     4,
	}
)

func (x PairingState) Enum() *PairingState {
	var p *PairingState

	p = new(PairingState)
	*p = x
	return p
}

func (x PairingState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (PairingState) Descriptor() protoreflect.EnumDescriptor {
	return file_mininaru_v1_mininaru_proto_enumTypes[0].Descriptor()
}

func (PairingState) Type() protoreflect.EnumType {
	return &file_mininaru_v1_mininaru_proto_enumTypes[0]
}

func (x PairingState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (PairingState) EnumDescriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{0}
}

type ApprovalChoice int32

const (
	ApprovalChoice_APPROVAL_CHOICE_UNSPECIFIED ApprovalChoice = 0
	ApprovalChoice_APPROVAL_CHOICE_DENY        ApprovalChoice = 1
	ApprovalChoice_APPROVAL_CHOICE_ONCE        ApprovalChoice = 2
	ApprovalChoice_APPROVAL_CHOICE_SESSION     ApprovalChoice = 3
)

var (
	ApprovalChoice_name = map[int32]string{
		0: "APPROVAL_CHOICE_UNSPECIFIED",
		1: "APPROVAL_CHOICE_DENY",
		2: "APPROVAL_CHOICE_ONCE",
		3: "APPROVAL_CHOICE_SESSION",
	}
	ApprovalChoice_value = map[string]int32{
		"APPROVAL_CHOICE_UNSPECIFIED": 0,
		"APPROVAL_CHOICE_DENY":        1,
		"APPROVAL_CHOICE_ONCE":        2,
		"APPROVAL_CHOICE_SESSION":     3,
	}
)

func (x ApprovalChoice) Enum() *ApprovalChoice {
	var p *ApprovalChoice

	p = new(ApprovalChoice)
	*p = x
	return p
}

func (x ApprovalChoice) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ApprovalChoice) Descriptor() protoreflect.EnumDescriptor {
	return file_mininaru_v1_mininaru_proto_enumTypes[1].Descriptor()
}

func (ApprovalChoice) Type() protoreflect.EnumType {
	return &file_mininaru_v1_mininaru_proto_enumTypes[1]
}

func (x ApprovalChoice) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

func (ApprovalChoice) EnumDescriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{1}
}

type Empty struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Empty) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = Empty{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[0]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Empty) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Empty) ProtoMessage() {}

func (x *Empty) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[0]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Empty) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{0}
}

type BeginPairingRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	PublicKey     []byte                 `protobuf:"bytes,1,opt,name=public_key,json=publicKey,proto3" json:"public_key,omitempty"`
	DeviceName    string                 `protobuf:"bytes,2,opt,name=device_name,json=deviceName,proto3" json:"device_name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BeginPairingRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = BeginPairingRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[1]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BeginPairingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BeginPairingRequest) ProtoMessage() {}

func (x *BeginPairingRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[1]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*BeginPairingRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{1}
}

func (x *BeginPairingRequest) GetPublicKey() []byte {
	if x != nil {
		return x.PublicKey
	}
	return nil
}

func (x *BeginPairingRequest) GetDeviceName() string {
	if x != nil {
		return x.DeviceName
	}
	return ""
}

type BeginPairingResponse struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	RequestId         string                 `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	PairingCode       string                 `protobuf:"bytes,2,opt,name=pairing_code,json=pairingCode,proto3" json:"pairing_code,omitempty"`
	ClientFingerprint string                 `protobuf:"bytes,3,opt,name=client_fingerprint,json=clientFingerprint,proto3" json:"client_fingerprint,omitempty"`
	ExpiresAtUnix     int64                  `protobuf:"varint,4,opt,name=expires_at_unix,json=expiresAtUnix,proto3" json:"expires_at_unix,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *BeginPairingResponse) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = BeginPairingResponse{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[2]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BeginPairingResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BeginPairingResponse) ProtoMessage() {}

func (x *BeginPairingResponse) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[2]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*BeginPairingResponse) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{2}
}

func (x *BeginPairingResponse) GetRequestId() string {
	if x != nil {
		return x.RequestId
	}
	return ""
}

func (x *BeginPairingResponse) GetPairingCode() string {
	if x != nil {
		return x.PairingCode
	}
	return ""
}

func (x *BeginPairingResponse) GetClientFingerprint() string {
	if x != nil {
		return x.ClientFingerprint
	}
	return ""
}

func (x *BeginPairingResponse) GetExpiresAtUnix() int64 {
	if x != nil {
		return x.ExpiresAtUnix
	}
	return 0
}

type WatchPairingRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RequestId     string                 `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *WatchPairingRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = WatchPairingRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[3]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *WatchPairingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WatchPairingRequest) ProtoMessage() {}

func (x *WatchPairingRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[3]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*WatchPairingRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{3}
}

func (x *WatchPairingRequest) GetRequestId() string {
	if x != nil {
		return x.RequestId
	}
	return ""
}

type PairingEvent struct {
	state                protoimpl.MessageState `protogen:"open.v1"`
	State                PairingState           `protobuf:"varint,1,opt,name=state,proto3,enum=mininaru.v1.PairingState" json:"state,omitempty"`
	ClientCertificatePem []byte                 `protobuf:"bytes,2,opt,name=client_certificate_pem,json=clientCertificatePem,proto3" json:"client_certificate_pem,omitempty"`
	CaCertificatePem     []byte                 `protobuf:"bytes,3,opt,name=ca_certificate_pem,json=caCertificatePem,proto3" json:"ca_certificate_pem,omitempty"`
	Error                string                 `protobuf:"bytes,4,opt,name=error,proto3" json:"error,omitempty"`
	unknownFields        protoimpl.UnknownFields
	sizeCache            protoimpl.SizeCache
}

func (x *PairingEvent) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = PairingEvent{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[4]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PairingEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PairingEvent) ProtoMessage() {}

func (x *PairingEvent) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[4]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*PairingEvent) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{4}
}

func (x *PairingEvent) GetState() PairingState {
	if x != nil {
		return x.State
	}
	return PairingState_PAIRING_STATE_UNSPECIFIED
}

func (x *PairingEvent) GetClientCertificatePem() []byte {
	if x != nil {
		return x.ClientCertificatePem
	}
	return nil
}

func (x *PairingEvent) GetCaCertificatePem() []byte {
	if x != nil {
		return x.CaCertificatePem
	}
	return nil
}

func (x *PairingEvent) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type Agent struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Model         string                 `protobuf:"bytes,3,opt,name=model,proto3" json:"model,omitempty"`
	Provider      string                 `protobuf:"bytes,4,opt,name=provider,proto3" json:"provider,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Agent) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = Agent{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[5]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Agent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Agent) ProtoMessage() {}

func (x *Agent) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[5]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Agent) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{5}
}

func (x *Agent) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Agent) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Agent) GetModel() string {
	if x != nil {
		return x.Model
	}
	return ""
}

func (x *Agent) GetProvider() string {
	if x != nil {
		return x.Provider
	}
	return ""
}

type Session struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	AgentId       string                 `protobuf:"bytes,2,opt,name=agent_id,json=agentId,proto3" json:"agent_id,omitempty"`
	Name          string                 `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Session) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = Session{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[6]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Session) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Session) ProtoMessage() {}

func (x *Session) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[6]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Session) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{6}
}

func (x *Session) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Session) GetAgentId() string {
	if x != nil {
		return x.AgentId
	}
	return ""
}

func (x *Session) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type Message struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	SessionId     string                 `protobuf:"bytes,2,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Role          string                 `protobuf:"bytes,3,opt,name=role,proto3" json:"role,omitempty"`
	Content       string                 `protobuf:"bytes,4,opt,name=content,proto3" json:"content,omitempty"`
	Reasoning     string                 `protobuf:"bytes,5,opt,name=reasoning,proto3" json:"reasoning,omitempty"`
	Status        string                 `protobuf:"bytes,6,opt,name=status,proto3" json:"status,omitempty"`
	Error         string                 `protobuf:"bytes,7,opt,name=error,proto3" json:"error,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Message) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = Message{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[7]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message) ProtoMessage() {}

func (x *Message) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[7]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Message) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{7}
}

func (x *Message) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Message) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

func (x *Message) GetRole() string {
	if x != nil {
		return x.Role
	}
	return ""
}

func (x *Message) GetContent() string {
	if x != nil {
		return x.Content
	}
	return ""
}

func (x *Message) GetReasoning() string {
	if x != nil {
		return x.Reasoning
	}
	return ""
}

func (x *Message) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *Message) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type ToolCall struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	CallId        string                 `protobuf:"bytes,2,opt,name=call_id,json=callId,proto3" json:"call_id,omitempty"`
	MessageId     string                 `protobuf:"bytes,3,opt,name=message_id,json=messageId,proto3" json:"message_id,omitempty"`
	Name          string                 `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty"`
	Arguments     string                 `protobuf:"bytes,5,opt,name=arguments,proto3" json:"arguments,omitempty"`
	Result        string                 `protobuf:"bytes,6,opt,name=result,proto3" json:"result,omitempty"`
	Status        string                 `protobuf:"bytes,7,opt,name=status,proto3" json:"status,omitempty"`
	Error         string                 `protobuf:"bytes,8,opt,name=error,proto3" json:"error,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ToolCall) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ToolCall{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[8]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ToolCall) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ToolCall) ProtoMessage() {}

func (x *ToolCall) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[8]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ToolCall) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{8}
}

func (x *ToolCall) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ToolCall) GetCallId() string {
	if x != nil {
		return x.CallId
	}
	return ""
}

func (x *ToolCall) GetMessageId() string {
	if x != nil {
		return x.MessageId
	}
	return ""
}

func (x *ToolCall) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ToolCall) GetArguments() string {
	if x != nil {
		return x.Arguments
	}
	return ""
}

func (x *ToolCall) GetResult() string {
	if x != nil {
		return x.Result
	}
	return ""
}

func (x *ToolCall) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *ToolCall) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type UsageLine struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	Kind             string                 `protobuf:"bytes,1,opt,name=kind,proto3" json:"kind,omitempty"`
	PromptTokens     int64                  `protobuf:"varint,2,opt,name=prompt_tokens,json=promptTokens,proto3" json:"prompt_tokens,omitempty"`
	CompletionTokens int64                  `protobuf:"varint,3,opt,name=completion_tokens,json=completionTokens,proto3" json:"completion_tokens,omitempty"`
	TotalTokens      int64                  `protobuf:"varint,4,opt,name=total_tokens,json=totalTokens,proto3" json:"total_tokens,omitempty"`
	CachedTokens     int64                  `protobuf:"varint,5,opt,name=cached_tokens,json=cachedTokens,proto3" json:"cached_tokens,omitempty"`
	CacheWriteTokens int64                  `protobuf:"varint,6,opt,name=cache_write_tokens,json=cacheWriteTokens,proto3" json:"cache_write_tokens,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *UsageLine) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = UsageLine{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[9]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *UsageLine) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UsageLine) ProtoMessage() {}

func (x *UsageLine) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[9]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*UsageLine) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{9}
}

func (x *UsageLine) GetKind() string {
	if x != nil {
		return x.Kind
	}
	return ""
}

func (x *UsageLine) GetPromptTokens() int64 {
	if x != nil {
		return x.PromptTokens
	}
	return 0
}

func (x *UsageLine) GetCompletionTokens() int64 {
	if x != nil {
		return x.CompletionTokens
	}
	return 0
}

func (x *UsageLine) GetTotalTokens() int64 {
	if x != nil {
		return x.TotalTokens
	}
	return 0
}

func (x *UsageLine) GetCachedTokens() int64 {
	if x != nil {
		return x.CachedTokens
	}
	return 0
}

func (x *UsageLine) GetCacheWriteTokens() int64 {
	if x != nil {
		return x.CacheWriteTokens
	}
	return 0
}

type Usage struct {
	state            protoimpl.MessageState `protogen:"open.v1"`
	SessionId        string                 `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Lines            []*UsageLine           `protobuf:"bytes,2,rep,name=lines,proto3" json:"lines,omitempty"`
	PromptTokens     int64                  `protobuf:"varint,3,opt,name=prompt_tokens,json=promptTokens,proto3" json:"prompt_tokens,omitempty"`
	CompletionTokens int64                  `protobuf:"varint,4,opt,name=completion_tokens,json=completionTokens,proto3" json:"completion_tokens,omitempty"`
	TotalTokens      int64                  `protobuf:"varint,5,opt,name=total_tokens,json=totalTokens,proto3" json:"total_tokens,omitempty"`
	CachedTokens     int64                  `protobuf:"varint,6,opt,name=cached_tokens,json=cachedTokens,proto3" json:"cached_tokens,omitempty"`
	CacheWriteTokens int64                  `protobuf:"varint,7,opt,name=cache_write_tokens,json=cacheWriteTokens,proto3" json:"cache_write_tokens,omitempty"`
	unknownFields    protoimpl.UnknownFields
	sizeCache        protoimpl.SizeCache
}

func (x *Usage) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = Usage{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[10]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Usage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Usage) ProtoMessage() {}

func (x *Usage) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[10]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Usage) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{10}
}

func (x *Usage) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

func (x *Usage) GetLines() []*UsageLine {
	if x != nil {
		return x.Lines
	}
	return nil
}

func (x *Usage) GetPromptTokens() int64 {
	if x != nil {
		return x.PromptTokens
	}
	return 0
}

func (x *Usage) GetCompletionTokens() int64 {
	if x != nil {
		return x.CompletionTokens
	}
	return 0
}

func (x *Usage) GetTotalTokens() int64 {
	if x != nil {
		return x.TotalTokens
	}
	return 0
}

func (x *Usage) GetCachedTokens() int64 {
	if x != nil {
		return x.CachedTokens
	}
	return 0
}

func (x *Usage) GetCacheWriteTokens() int64 {
	if x != nil {
		return x.CacheWriteTokens
	}
	return 0
}

type ListAgentsRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListAgentsRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ListAgentsRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[11]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListAgentsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListAgentsRequest) ProtoMessage() {}

func (x *ListAgentsRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[11]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ListAgentsRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{11}
}

type ListAgentsResponse struct {
	state          protoimpl.MessageState `protogen:"open.v1"`
	Agents         []*Agent               `protobuf:"bytes,1,rep,name=agents,proto3" json:"agents,omitempty"`
	DefaultAgentId string                 `protobuf:"bytes,2,opt,name=default_agent_id,json=defaultAgentId,proto3" json:"default_agent_id,omitempty"`
	unknownFields  protoimpl.UnknownFields
	sizeCache      protoimpl.SizeCache
}

func (x *ListAgentsResponse) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ListAgentsResponse{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[12]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListAgentsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListAgentsResponse) ProtoMessage() {}

func (x *ListAgentsResponse) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[12]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ListAgentsResponse) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{12}
}

func (x *ListAgentsResponse) GetAgents() []*Agent {
	if x != nil {
		return x.Agents
	}
	return nil
}

func (x *ListAgentsResponse) GetDefaultAgentId() string {
	if x != nil {
		return x.DefaultAgentId
	}
	return ""
}

type Skill struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Description   string                 `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
	Scope         string                 `protobuf:"bytes,3,opt,name=scope,proto3" json:"scope,omitempty"`
	Body          string                 `protobuf:"bytes,4,opt,name=body,proto3" json:"body,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Skill) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = Skill{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[13]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Skill) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Skill) ProtoMessage() {}

func (x *Skill) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[13]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*Skill) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{13}
}

func (x *Skill) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Skill) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *Skill) GetScope() string {
	if x != nil {
		return x.Scope
	}
	return ""
}

func (x *Skill) GetBody() string {
	if x != nil {
		return x.Body
	}
	return ""
}

type ListSkillsRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListSkillsRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ListSkillsRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[14]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListSkillsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListSkillsRequest) ProtoMessage() {}

func (x *ListSkillsRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[14]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ListSkillsRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{14}
}

type ListSkillsResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Skills        []*Skill               `protobuf:"bytes,1,rep,name=skills,proto3" json:"skills,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListSkillsResponse) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ListSkillsResponse{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[15]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListSkillsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListSkillsResponse) ProtoMessage() {}

func (x *ListSkillsResponse) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[15]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ListSkillsResponse) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{15}
}

func (x *ListSkillsResponse) GetSkills() []*Skill {
	if x != nil {
		return x.Skills
	}
	return nil
}

type GetSkillRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetSkillRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = GetSkillRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[16]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetSkillRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSkillRequest) ProtoMessage() {}

func (x *GetSkillRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[16]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*GetSkillRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{16}
}

func (x *GetSkillRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type ListSessionsRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Agent         string                 `protobuf:"bytes,1,opt,name=agent,proto3" json:"agent,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListSessionsRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ListSessionsRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[17]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListSessionsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListSessionsRequest) ProtoMessage() {}

func (x *ListSessionsRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[17]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ListSessionsRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{17}
}

func (x *ListSessionsRequest) GetAgent() string {
	if x != nil {
		return x.Agent
	}
	return ""
}

type ListSessionsResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Sessions      []*Session             `protobuf:"bytes,1,rep,name=sessions,proto3" json:"sessions,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListSessionsResponse) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ListSessionsResponse{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[18]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListSessionsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListSessionsResponse) ProtoMessage() {}

func (x *ListSessionsResponse) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[18]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ListSessionsResponse) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{18}
}

func (x *ListSessionsResponse) GetSessions() []*Session {
	if x != nil {
		return x.Sessions
	}
	return nil
}

type CreateSessionRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Agent         string                 `protobuf:"bytes,1,opt,name=agent,proto3" json:"agent,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CreateSessionRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = CreateSessionRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[19]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CreateSessionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateSessionRequest) ProtoMessage() {}

func (x *CreateSessionRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[19]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*CreateSessionRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{19}
}

func (x *CreateSessionRequest) GetAgent() string {
	if x != nil {
		return x.Agent
	}
	return ""
}

func (x *CreateSessionRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type GetSessionRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	SessionId     string                 `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetSessionRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = GetSessionRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[20]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetSessionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetSessionRequest) ProtoMessage() {}

func (x *GetSessionRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[20]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*GetSessionRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{20}
}

func (x *GetSessionRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

type SessionDetail struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Session       *Session               `protobuf:"bytes,1,opt,name=session,proto3" json:"session,omitempty"`
	Agent         *Agent                 `protobuf:"bytes,2,opt,name=agent,proto3" json:"agent,omitempty"`
	Messages      []*Message             `protobuf:"bytes,3,rep,name=messages,proto3" json:"messages,omitempty"`
	ContextTokens int64                  `protobuf:"varint,4,opt,name=context_tokens,json=contextTokens,proto3" json:"context_tokens,omitempty"`
	ContextWindow int64                  `protobuf:"varint,5,opt,name=context_window,json=contextWindow,proto3" json:"context_window,omitempty"`
	ContextKnown  bool                   `protobuf:"varint,6,opt,name=context_known,json=contextKnown,proto3" json:"context_known,omitempty"`
	ToolCalls     []*ToolCall            `protobuf:"bytes,7,rep,name=tool_calls,json=toolCalls,proto3" json:"tool_calls,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SessionDetail) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = SessionDetail{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[21]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SessionDetail) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SessionDetail) ProtoMessage() {}

func (x *SessionDetail) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[21]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*SessionDetail) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{21}
}

func (x *SessionDetail) GetSession() *Session {
	if x != nil {
		return x.Session
	}
	return nil
}

func (x *SessionDetail) GetAgent() *Agent {
	if x != nil {
		return x.Agent
	}
	return nil
}

func (x *SessionDetail) GetMessages() []*Message {
	if x != nil {
		return x.Messages
	}
	return nil
}

func (x *SessionDetail) GetContextTokens() int64 {
	if x != nil {
		return x.ContextTokens
	}
	return 0
}

func (x *SessionDetail) GetContextWindow() int64 {
	if x != nil {
		return x.ContextWindow
	}
	return 0
}

func (x *SessionDetail) GetContextKnown() bool {
	if x != nil {
		return x.ContextKnown
	}
	return false
}

func (x *SessionDetail) GetToolCalls() []*ToolCall {
	if x != nil {
		return x.ToolCalls
	}
	return nil
}

type RenameSessionRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	SessionId     string                 `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RenameSessionRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = RenameSessionRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[22]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RenameSessionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RenameSessionRequest) ProtoMessage() {}

func (x *RenameSessionRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[22]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*RenameSessionRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{22}
}

func (x *RenameSessionRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

func (x *RenameSessionRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type DeleteSessionRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	SessionId     string                 `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *DeleteSessionRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = DeleteSessionRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[23]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *DeleteSessionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteSessionRequest) ProtoMessage() {}

func (x *DeleteSessionRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[23]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*DeleteSessionRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{23}
}

func (x *DeleteSessionRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

type GetUsageRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	SessionId     string                 `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetUsageRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = GetUsageRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[24]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetUsageRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetUsageRequest) ProtoMessage() {}

func (x *GetUsageRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[24]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*GetUsageRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{24}
}

func (x *GetUsageRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

type CompactSessionRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	SessionId     string                 `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CompactSessionRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = CompactSessionRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[25]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CompactSessionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CompactSessionRequest) ProtoMessage() {}

func (x *CompactSessionRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[25]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*CompactSessionRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{25}
}

func (x *CompactSessionRequest) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

type CompactSessionResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Compacted     bool                   `protobuf:"varint,1,opt,name=compacted,proto3" json:"compacted,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CompactSessionResponse) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = CompactSessionResponse{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[26]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CompactSessionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CompactSessionResponse) ProtoMessage() {}

func (x *CompactSessionResponse) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[26]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*CompactSessionResponse) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{26}
}

func (x *CompactSessionResponse) GetCompacted() bool {
	if x != nil {
		return x.Compacted
	}
	return false
}

type ChatStart struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	SessionId     string                 `protobuf:"bytes,1,opt,name=session_id,json=sessionId,proto3" json:"session_id,omitempty"`
	Content       string                 `protobuf:"bytes,2,opt,name=content,proto3" json:"content,omitempty"`
	Thinking      string                 `protobuf:"bytes,3,opt,name=thinking,proto3" json:"thinking,omitempty"`
	Tools         []*ToolDefinition      `protobuf:"bytes,4,rep,name=tools,proto3" json:"tools,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChatStart) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ChatStart{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[27]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChatStart) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatStart) ProtoMessage() {}

func (x *ChatStart) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[27]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ChatStart) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{27}
}

func (x *ChatStart) GetSessionId() string {
	if x != nil {
		return x.SessionId
	}
	return ""
}

func (x *ChatStart) GetContent() string {
	if x != nil {
		return x.Content
	}
	return ""
}

func (x *ChatStart) GetThinking() string {
	if x != nil {
		return x.Thinking
	}
	return ""
}

func (x *ChatStart) GetTools() []*ToolDefinition {
	if x != nil {
		return x.Tools
	}
	return nil
}

type ToolDefinition struct {
	state          protoimpl.MessageState `protogen:"open.v1"`
	Name           string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Description    string                 `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
	ParametersJson string                 `protobuf:"bytes,3,opt,name=parameters_json,json=parametersJson,proto3" json:"parameters_json,omitempty"`
	Permission     string                 `protobuf:"bytes,4,opt,name=permission,proto3" json:"permission,omitempty"`
	unknownFields  protoimpl.UnknownFields
	sizeCache      protoimpl.SizeCache
}

func (x *ToolDefinition) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ToolDefinition{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[28]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ToolDefinition) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ToolDefinition) ProtoMessage() {}

func (x *ToolDefinition) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[28]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ToolDefinition) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{28}
}

func (x *ToolDefinition) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ToolDefinition) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *ToolDefinition) GetParametersJson() string {
	if x != nil {
		return x.ParametersJson
	}
	return ""
}

func (x *ToolDefinition) GetPermission() string {
	if x != nil {
		return x.Permission
	}
	return ""
}

type ToolResult struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RequestId     string                 `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	Result        string                 `protobuf:"bytes,2,opt,name=result,proto3" json:"result,omitempty"`
	Error         string                 `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ToolResult) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ToolResult{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[29]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ToolResult) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ToolResult) ProtoMessage() {}

func (x *ToolResult) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[29]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ToolResult) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{29}
}

func (x *ToolResult) GetRequestId() string {
	if x != nil {
		return x.RequestId
	}
	return ""
}

func (x *ToolResult) GetResult() string {
	if x != nil {
		return x.Result
	}
	return ""
}

func (x *ToolResult) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type ApprovalDecision struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RequestId     string                 `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	Choice        ApprovalChoice         `protobuf:"varint,2,opt,name=choice,proto3,enum=mininaru.v1.ApprovalChoice" json:"choice,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ApprovalDecision) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ApprovalDecision{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[30]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ApprovalDecision) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApprovalDecision) ProtoMessage() {}

func (x *ApprovalDecision) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[30]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ApprovalDecision) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{30}
}

func (x *ApprovalDecision) GetRequestId() string {
	if x != nil {
		return x.RequestId
	}
	return ""
}

func (x *ApprovalDecision) GetChoice() ApprovalChoice {
	if x != nil {
		return x.Choice
	}
	return ApprovalChoice_APPROVAL_CHOICE_UNSPECIFIED
}

type ChatClientEvent struct {
	state         protoimpl.MessageState  `protogen:"open.v1"`
	Event         isChatClientEvent_Event `protobuf_oneof:"event"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChatClientEvent) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ChatClientEvent{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[31]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChatClientEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatClientEvent) ProtoMessage() {}

func (x *ChatClientEvent) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[31]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ChatClientEvent) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{31}
}

func (x *ChatClientEvent) GetEvent() isChatClientEvent_Event {
	if x != nil {
		return x.Event
	}
	return nil
}

func (x *ChatClientEvent) GetStart() *ChatStart {
	var (
		xValue *ChatClientEvent_Start
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatClientEvent_Start); ok {
			return xValue.Start
		}
	}
	return nil
}

func (x *ChatClientEvent) GetApproval() *ApprovalDecision {
	var (
		xValue *ChatClientEvent_Approval
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatClientEvent_Approval); ok {
			return xValue.Approval
		}
	}
	return nil
}

func (x *ChatClientEvent) GetCancel() *Empty {
	var (
		xValue *ChatClientEvent_Cancel
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatClientEvent_Cancel); ok {
			return xValue.Cancel
		}
	}
	return nil
}

func (x *ChatClientEvent) GetToolResult() *ToolResult {
	var (
		xValue *ChatClientEvent_ToolResult
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatClientEvent_ToolResult); ok {
			return xValue.ToolResult
		}
	}
	return nil
}

type isChatClientEvent_Event interface {
	isChatClientEvent_Event()
}

type ChatClientEvent_Start struct {
	Start *ChatStart `protobuf:"bytes,1,opt,name=start,proto3,oneof"`
}

type ChatClientEvent_Approval struct {
	Approval *ApprovalDecision `protobuf:"bytes,2,opt,name=approval,proto3,oneof"`
}

type ChatClientEvent_Cancel struct {
	Cancel *Empty `protobuf:"bytes,3,opt,name=cancel,proto3,oneof"`
}

type ChatClientEvent_ToolResult struct {
	ToolResult *ToolResult `protobuf:"bytes,4,opt,name=tool_result,json=toolResult,proto3,oneof"`
}

func (*ChatClientEvent_Start) isChatClientEvent_Event() {}

func (*ChatClientEvent_Approval) isChatClientEvent_Event() {}

func (*ChatClientEvent_Cancel) isChatClientEvent_Event() {}

func (*ChatClientEvent_ToolResult) isChatClientEvent_Event() {}

type ChatStarted struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	TurnId        string                 `protobuf:"bytes,1,opt,name=turn_id,json=turnId,proto3" json:"turn_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChatStarted) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ChatStarted{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[32]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChatStarted) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatStarted) ProtoMessage() {}

func (x *ChatStarted) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[32]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ChatStarted) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{32}
}

func (x *ChatStarted) GetTurnId() string {
	if x != nil {
		return x.TurnId
	}
	return ""
}

type TextDelta struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Text          string                 `protobuf:"bytes,1,opt,name=text,proto3" json:"text,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TextDelta) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = TextDelta{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[33]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TextDelta) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TextDelta) ProtoMessage() {}

func (x *TextDelta) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[33]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*TextDelta) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{33}
}

func (x *TextDelta) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

type ToolEvent struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Phase         string                 `protobuf:"bytes,1,opt,name=phase,proto3" json:"phase,omitempty"`
	CallId        string                 `protobuf:"bytes,2,opt,name=call_id,json=callId,proto3" json:"call_id,omitempty"`
	Name          string                 `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	Arguments     string                 `protobuf:"bytes,4,opt,name=arguments,proto3" json:"arguments,omitempty"`
	Result        string                 `protobuf:"bytes,5,opt,name=result,proto3" json:"result,omitempty"`
	Status        string                 `protobuf:"bytes,6,opt,name=status,proto3" json:"status,omitempty"`
	Error         string                 `protobuf:"bytes,7,opt,name=error,proto3" json:"error,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ToolEvent) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ToolEvent{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[34]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ToolEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ToolEvent) ProtoMessage() {}

func (x *ToolEvent) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[34]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ToolEvent) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{34}
}

func (x *ToolEvent) GetPhase() string {
	if x != nil {
		return x.Phase
	}
	return ""
}

func (x *ToolEvent) GetCallId() string {
	if x != nil {
		return x.CallId
	}
	return ""
}

func (x *ToolEvent) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ToolEvent) GetArguments() string {
	if x != nil {
		return x.Arguments
	}
	return ""
}

func (x *ToolEvent) GetResult() string {
	if x != nil {
		return x.Result
	}
	return ""
}

func (x *ToolEvent) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *ToolEvent) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type ApprovalRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RequestId     string                 `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	ToolName      string                 `protobuf:"bytes,2,opt,name=tool_name,json=toolName,proto3" json:"tool_name,omitempty"`
	Arguments     string                 `protobuf:"bytes,3,opt,name=arguments,proto3" json:"arguments,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ApprovalRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ApprovalRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[35]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ApprovalRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApprovalRequest) ProtoMessage() {}

func (x *ApprovalRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[35]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ApprovalRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{35}
}

func (x *ApprovalRequest) GetRequestId() string {
	if x != nil {
		return x.RequestId
	}
	return ""
}

func (x *ApprovalRequest) GetToolName() string {
	if x != nil {
		return x.ToolName
	}
	return ""
}

func (x *ApprovalRequest) GetArguments() string {
	if x != nil {
		return x.Arguments
	}
	return ""
}

type ToolRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RequestId     string                 `protobuf:"bytes,1,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
	ToolName      string                 `protobuf:"bytes,2,opt,name=tool_name,json=toolName,proto3" json:"tool_name,omitempty"`
	Arguments     string                 `protobuf:"bytes,3,opt,name=arguments,proto3" json:"arguments,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ToolRequest) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ToolRequest{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[36]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ToolRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ToolRequest) ProtoMessage() {}

func (x *ToolRequest) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[36]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ToolRequest) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{36}
}

func (x *ToolRequest) GetRequestId() string {
	if x != nil {
		return x.RequestId
	}
	return ""
}

func (x *ToolRequest) GetToolName() string {
	if x != nil {
		return x.ToolName
	}
	return ""
}

func (x *ToolRequest) GetArguments() string {
	if x != nil {
		return x.Arguments
	}
	return ""
}

type ChatCompleted struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Message       *Message               `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	Usage         *Usage                 `protobuf:"bytes,2,opt,name=usage,proto3" json:"usage,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChatCompleted) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ChatCompleted{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[37]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChatCompleted) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatCompleted) ProtoMessage() {}

func (x *ChatCompleted) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[37]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ChatCompleted) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{37}
}

func (x *ChatCompleted) GetMessage() *Message {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *ChatCompleted) GetUsage() *Usage {
	if x != nil {
		return x.Usage
	}
	return nil
}

type ChatFailed struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Code          string                 `protobuf:"bytes,1,opt,name=code,proto3" json:"code,omitempty"`
	Message       string                 `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChatFailed) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ChatFailed{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[38]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChatFailed) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatFailed) ProtoMessage() {}

func (x *ChatFailed) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[38]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ChatFailed) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{38}
}

func (x *ChatFailed) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

func (x *ChatFailed) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type ChatServerEvent struct {
	state         protoimpl.MessageState  `protogen:"open.v1"`
	Event         isChatServerEvent_Event `protobuf_oneof:"event"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ChatServerEvent) Reset() {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	*x = ChatServerEvent{}
	mi = &file_mininaru_v1_mininaru_proto_msgTypes[39]
	ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ChatServerEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatServerEvent) ProtoMessage() {}

func (x *ChatServerEvent) ProtoReflect() protoreflect.Message {
	var (
		mi *protoimpl.
			MessageInfo
		ms messageState
	)

	mi = &file_mininaru_v1_mininaru_proto_msgTypes[39]
	if x != nil {
		ms = protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

func (*ChatServerEvent) Descriptor() ([]byte, []int) {
	return file_mininaru_v1_mininaru_proto_rawDescGZIP(), []int{39}
}

func (x *ChatServerEvent) GetEvent() isChatServerEvent_Event {
	if x != nil {
		return x.Event
	}
	return nil
}

func (x *ChatServerEvent) GetStarted() *ChatStarted {
	var (
		xValue *ChatServerEvent_Started
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatServerEvent_Started); ok {
			return xValue.Started
		}
	}
	return nil
}

func (x *ChatServerEvent) GetContent() *TextDelta {
	var (
		xValue *ChatServerEvent_Content
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatServerEvent_Content); ok {
			return xValue.Content
		}
	}
	return nil
}

func (x *ChatServerEvent) GetReasoning() *TextDelta {
	var (
		xValue *ChatServerEvent_Reasoning
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatServerEvent_Reasoning); ok {
			return xValue.Reasoning
		}
	}
	return nil
}

func (x *ChatServerEvent) GetTool() *ToolEvent {
	var (
		xValue *ChatServerEvent_Tool
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatServerEvent_Tool); ok {
			return xValue.Tool
		}
	}
	return nil
}

func (x *ChatServerEvent) GetApproval() *ApprovalRequest {
	var (
		xValue *ChatServerEvent_Approval
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatServerEvent_Approval); ok {
			return xValue.Approval
		}
	}
	return nil
}

func (x *ChatServerEvent) GetCompleted() *ChatCompleted {
	var (
		xValue *ChatServerEvent_Completed
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatServerEvent_Completed); ok {
			return xValue.Completed
		}
	}
	return nil
}

func (x *ChatServerEvent) GetFailed() *ChatFailed {
	var (
		xValue *ChatServerEvent_Failed
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatServerEvent_Failed); ok {
			return xValue.Failed
		}
	}
	return nil
}

func (x *ChatServerEvent) GetToolRequest() *ToolRequest {
	var (
		xValue *ChatServerEvent_ToolRequest
		ok     bool
	)

	if x != nil {
		if xValue, ok = x.Event.(*ChatServerEvent_ToolRequest); ok {
			return xValue.ToolRequest
		}
	}
	return nil
}

type isChatServerEvent_Event interface {
	isChatServerEvent_Event()
}

type ChatServerEvent_Started struct {
	Started *ChatStarted `protobuf:"bytes,1,opt,name=started,proto3,oneof"`
}

type ChatServerEvent_Content struct {
	Content *TextDelta `protobuf:"bytes,2,opt,name=content,proto3,oneof"`
}

type ChatServerEvent_Reasoning struct {
	Reasoning *TextDelta `protobuf:"bytes,3,opt,name=reasoning,proto3,oneof"`
}

type ChatServerEvent_Tool struct {
	Tool *ToolEvent `protobuf:"bytes,4,opt,name=tool,proto3,oneof"`
}

type ChatServerEvent_Approval struct {
	Approval *ApprovalRequest `protobuf:"bytes,5,opt,name=approval,proto3,oneof"`
}

type ChatServerEvent_Completed struct {
	Completed *ChatCompleted `protobuf:"bytes,6,opt,name=completed,proto3,oneof"`
}

type ChatServerEvent_Failed struct {
	Failed *ChatFailed `protobuf:"bytes,7,opt,name=failed,proto3,oneof"`
}

type ChatServerEvent_ToolRequest struct {
	ToolRequest *ToolRequest `protobuf:"bytes,8,opt,name=tool_request,json=toolRequest,proto3,oneof"`
}

func (*ChatServerEvent_Started) isChatServerEvent_Event() {}

func (*ChatServerEvent_Content) isChatServerEvent_Event() {}

func (*ChatServerEvent_Reasoning) isChatServerEvent_Event() {}

func (*ChatServerEvent_Tool) isChatServerEvent_Event() {}

func (*ChatServerEvent_Approval) isChatServerEvent_Event() {}

func (*ChatServerEvent_Completed) isChatServerEvent_Event() {}

func (*ChatServerEvent_Failed) isChatServerEvent_Event() {}

func (*ChatServerEvent_ToolRequest) isChatServerEvent_Event() {}

var File_mininaru_v1_mininaru_proto protoreflect.FileDescriptor

const file_mininaru_v1_mininaru_proto_rawDesc = "" +
	"\n" +
	"\x1amininaru/v1/mininaru.proto\x12\vmininaru.v1\"\a\n" +
	"\x05Empty\"U\n" +
	"\x13BeginPairingRequest\x12\x1d\n" +
	"\n" +
	"public_key\x18\x01 \x01(\fR\tpublicKey\x12\x1f\n" +
	"\vdevice_name\x18\x02 \x01(\tR\n" +
	"deviceName\"\xaf\x01\n" +
	"\x14BeginPairingResponse\x12\x1d\n" +
	"\n" +
	"request_id\x18\x01 \x01(\tR\trequestId\x12!\n" +
	"\fpairing_code\x18\x02 \x01(\tR\vpairingCode\x12-\n" +
	"\x12client_fingerprint\x18\x03 \x01(\tR\x11clientFingerprint\x12&\n" +
	"\x0fexpires_at_unix\x18\x04 \x01(\x03R\rexpiresAtUnix\"4\n" +
	"\x13WatchPairingRequest\x12\x1d\n" +
	"\n" +
	"request_id\x18\x01 \x01(\tR\trequestId\"\xb9\x01\n" +
	"\fPairingEvent\x12/\n" +
	"\x05state\x18\x01 \x01(\x0e2\x19.mininaru.v1.PairingStateR\x05state\x124\n" +
	"\x16client_certificate_pem\x18\x02 \x01(\fR\x14clientCertificatePem\x12,\n" +
	"\x12ca_certificate_pem\x18\x03 \x01(\fR\x10caCertificatePem\x12\x14\n" +
	"\x05error\x18\x04 \x01(\tR\x05error\"]\n" +
	"\x05Agent\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\x12\x12\n" +
	"\x04name\x18\x02 \x01(\tR\x04name\x12\x14\n" +
	"\x05model\x18\x03 \x01(\tR\x05model\x12\x1a\n" +
	"\bprovider\x18\x04 \x01(\tR\bprovider\"H\n" +
	"\aSession\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\x12\x19\n" +
	"\bagent_id\x18\x02 \x01(\tR\aagentId\x12\x12\n" +
	"\x04name\x18\x03 \x01(\tR\x04name\"\xb2\x01\n" +
	"\aMessage\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\x12\x1d\n" +
	"\n" +
	"session_id\x18\x02 \x01(\tR\tsessionId\x12\x12\n" +
	"\x04role\x18\x03 \x01(\tR\x04role\x12\x18\n" +
	"\acontent\x18\x04 \x01(\tR\acontent\x12\x1c\n" +
	"\treasoning\x18\x05 \x01(\tR\treasoning\x12\x16\n" +
	"\x06status\x18\x06 \x01(\tR\x06status\x12\x14\n" +
	"\x05error\x18\a \x01(\tR\x05error\"\xca\x01\n" +
	"\bToolCall\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\x12\x17\n" +
	"\acall_id\x18\x02 \x01(\tR\x06callId\x12\x1d\n" +
	"\n" +
	"message_id\x18\x03 \x01(\tR\tmessageId\x12\x12\n" +
	"\x04name\x18\x04 \x01(\tR\x04name\x12\x1c\n" +
	"\targuments\x18\x05 \x01(\tR\targuments\x12\x16\n" +
	"\x06result\x18\x06 \x01(\tR\x06result\x12\x16\n" +
	"\x06status\x18\a \x01(\tR\x06status\x12\x14\n" +
	"\x05error\x18\b \x01(\tR\x05error\"\xe7\x01\n" +
	"\tUsageLine\x12\x12\n" +
	"\x04kind\x18\x01 \x01(\tR\x04kind\x12#\n" +
	"\rprompt_tokens\x18\x02 \x01(\x03R\fpromptTokens\x12+\n" +
	"\x11completion_tokens\x18\x03 \x01(\x03R\x10completionTokens\x12!\n" +
	"\ftotal_tokens\x18\x04 \x01(\x03R\vtotalTokens\x12#\n" +
	"\rcached_tokens\x18\x05 \x01(\x03R\fcachedTokens\x12,\n" +
	"\x12cache_write_tokens\x18\x06 \x01(\x03R\x10cacheWriteTokens\"\x9c\x02\n" +
	"\x05Usage\x12\x1d\n" +
	"\n" +
	"session_id\x18\x01 \x01(\tR\tsessionId\x12,\n" +
	"\x05lines\x18\x02 \x03(\v2\x16.mininaru.v1.UsageLineR\x05lines\x12#\n" +
	"\rprompt_tokens\x18\x03 \x01(\x03R\fpromptTokens\x12+\n" +
	"\x11completion_tokens\x18\x04 \x01(\x03R\x10completionTokens\x12!\n" +
	"\ftotal_tokens\x18\x05 \x01(\x03R\vtotalTokens\x12#\n" +
	"\rcached_tokens\x18\x06 \x01(\x03R\fcachedTokens\x12,\n" +
	"\x12cache_write_tokens\x18\a \x01(\x03R\x10cacheWriteTokens\"\x13\n" +
	"\x11ListAgentsRequest\"j\n" +
	"\x12ListAgentsResponse\x12*\n" +
	"\x06agents\x18\x01 \x03(\v2\x12.mininaru.v1.AgentR\x06agents\x12(\n" +
	"\x10default_agent_id\x18\x02 \x01(\tR\x0edefaultAgentId\"g\n" +
	"\x05Skill\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12 \n" +
	"\vdescription\x18\x02 \x01(\tR\vdescription\x12\x14\n" +
	"\x05scope\x18\x03 \x01(\tR\x05scope\x12\x12\n" +
	"\x04body\x18\x04 \x01(\tR\x04body\"\x13\n" +
	"\x11ListSkillsRequest\"@\n" +
	"\x12ListSkillsResponse\x12*\n" +
	"\x06skills\x18\x01 \x03(\v2\x12.mininaru.v1.SkillR\x06skills\"%\n" +
	"\x0fGetSkillRequest\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\"+\n" +
	"\x13ListSessionsRequest\x12\x14\n" +
	"\x05agent\x18\x01 \x01(\tR\x05agent\"H\n" +
	"\x14ListSessionsResponse\x120\n" +
	"\bsessions\x18\x01 \x03(\v2\x14.mininaru.v1.SessionR\bsessions\"@\n" +
	"\x14CreateSessionRequest\x12\x14\n" +
	"\x05agent\x18\x01 \x01(\tR\x05agent\x12\x12\n" +
	"\x04name\x18\x02 \x01(\tR\x04name\"2\n" +
	"\x11GetSessionRequest\x12\x1d\n" +
	"\n" +
	"session_id\x18\x01 \x01(\tR\tsessionId\"\xc4\x02\n" +
	"\rSessionDetail\x12.\n" +
	"\asession\x18\x01 \x01(\v2\x14.mininaru.v1.SessionR\asession\x12(\n" +
	"\x05agent\x18\x02 \x01(\v2\x12.mininaru.v1.AgentR\x05agent\x120\n" +
	"\bmessages\x18\x03 \x03(\v2\x14.mininaru.v1.MessageR\bmessages\x12%\n" +
	"\x0econtext_tokens\x18\x04 \x01(\x03R\rcontextTokens\x12%\n" +
	"\x0econtext_window\x18\x05 \x01(\x03R\rcontextWindow\x12#\n" +
	"\rcontext_known\x18\x06 \x01(\bR\fcontextKnown\x124\n" +
	"\n" +
	"tool_calls\x18\a \x03(\v2\x15.mininaru.v1.ToolCallR\ttoolCalls\"I\n" +
	"\x14RenameSessionRequest\x12\x1d\n" +
	"\n" +
	"session_id\x18\x01 \x01(\tR\tsessionId\x12\x12\n" +
	"\x04name\x18\x02 \x01(\tR\x04name\"5\n" +
	"\x14DeleteSessionRequest\x12\x1d\n" +
	"\n" +
	"session_id\x18\x01 \x01(\tR\tsessionId\"0\n" +
	"\x0fGetUsageRequest\x12\x1d\n" +
	"\n" +
	"session_id\x18\x01 \x01(\tR\tsessionId\"6\n" +
	"\x15CompactSessionRequest\x12\x1d\n" +
	"\n" +
	"session_id\x18\x01 \x01(\tR\tsessionId\"6\n" +
	"\x16CompactSessionResponse\x12\x1c\n" +
	"\tcompacted\x18\x01 \x01(\bR\tcompacted\"\x93\x01\n" +
	"\tChatStart\x12\x1d\n" +
	"\n" +
	"session_id\x18\x01 \x01(\tR\tsessionId\x12\x18\n" +
	"\acontent\x18\x02 \x01(\tR\acontent\x12\x1a\n" +
	"\bthinking\x18\x03 \x01(\tR\bthinking\x121\n" +
	"\x05tools\x18\x04 \x03(\v2\x1b.mininaru.v1.ToolDefinitionR\x05tools\"\x8f\x01\n" +
	"\x0eToolDefinition\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\x12 \n" +
	"\vdescription\x18\x02 \x01(\tR\vdescription\x12'\n" +
	"\x0fparameters_json\x18\x03 \x01(\tR\x0eparametersJson\x12\x1e\n" +
	"\n" +
	"permission\x18\x04 \x01(\tR\n" +
	"permission\"Y\n" +
	"\n" +
	"ToolResult\x12\x1d\n" +
	"\n" +
	"request_id\x18\x01 \x01(\tR\trequestId\x12\x16\n" +
	"\x06result\x18\x02 \x01(\tR\x06result\x12\x14\n" +
	"\x05error\x18\x03 \x01(\tR\x05error\"f\n" +
	"\x10ApprovalDecision\x12\x1d\n" +
	"\n" +
	"request_id\x18\x01 \x01(\tR\trequestId\x123\n" +
	"\x06choice\x18\x02 \x01(\x0e2\x1b.mininaru.v1.ApprovalChoiceR\x06choice\"\xf1\x01\n" +
	"\x0fChatClientEvent\x12.\n" +
	"\x05start\x18\x01 \x01(\v2\x16.mininaru.v1.ChatStartH\x00R\x05start\x12;\n" +
	"\bapproval\x18\x02 \x01(\v2\x1d.mininaru.v1.ApprovalDecisionH\x00R\bapproval\x12,\n" +
	"\x06cancel\x18\x03 \x01(\v2\x12.mininaru.v1.EmptyH\x00R\x06cancel\x12:\n" +
	"\vtool_result\x18\x04 \x01(\v2\x17.mininaru.v1.ToolResultH\x00R\n" +
	"toolResultB\a\n" +
	"\x05event\"&\n" +
	"\vChatStarted\x12\x17\n" +
	"\aturn_id\x18\x01 \x01(\tR\x06turnId\"\x1f\n" +
	"\tTextDelta\x12\x12\n" +
	"\x04text\x18\x01 \x01(\tR\x04text\"\xb2\x01\n" +
	"\tToolEvent\x12\x14\n" +
	"\x05phase\x18\x01 \x01(\tR\x05phase\x12\x17\n" +
	"\acall_id\x18\x02 \x01(\tR\x06callId\x12\x12\n" +
	"\x04name\x18\x03 \x01(\tR\x04name\x12\x1c\n" +
	"\targuments\x18\x04 \x01(\tR\targuments\x12\x16\n" +
	"\x06result\x18\x05 \x01(\tR\x06result\x12\x16\n" +
	"\x06status\x18\x06 \x01(\tR\x06status\x12\x14\n" +
	"\x05error\x18\a \x01(\tR\x05error\"k\n" +
	"\x0fApprovalRequest\x12\x1d\n" +
	"\n" +
	"request_id\x18\x01 \x01(\tR\trequestId\x12\x1b\n" +
	"\ttool_name\x18\x02 \x01(\tR\btoolName\x12\x1c\n" +
	"\targuments\x18\x03 \x01(\tR\targuments\"g\n" +
	"\vToolRequest\x12\x1d\n" +
	"\n" +
	"request_id\x18\x01 \x01(\tR\trequestId\x12\x1b\n" +
	"\ttool_name\x18\x02 \x01(\tR\btoolName\x12\x1c\n" +
	"\targuments\x18\x03 \x01(\tR\targuments\"i\n" +
	"\rChatCompleted\x12.\n" +
	"\amessage\x18\x01 \x01(\v2\x14.mininaru.v1.MessageR\amessage\x12(\n" +
	"\x05usage\x18\x02 \x01(\v2\x12.mininaru.v1.UsageR\x05usage\":\n" +
	"\n" +
	"ChatFailed\x12\x12\n" +
	"\x04code\x18\x01 \x01(\tR\x04code\x12\x18\n" +
	"\amessage\x18\x02 \x01(\tR\amessage\"\xd4\x03\n" +
	"\x0fChatServerEvent\x124\n" +
	"\astarted\x18\x01 \x01(\v2\x18.mininaru.v1.ChatStartedH\x00R\astarted\x122\n" +
	"\acontent\x18\x02 \x01(\v2\x16.mininaru.v1.TextDeltaH\x00R\acontent\x126\n" +
	"\treasoning\x18\x03 \x01(\v2\x16.mininaru.v1.TextDeltaH\x00R\treasoning\x12,\n" +
	"\x04tool\x18\x04 \x01(\v2\x16.mininaru.v1.ToolEventH\x00R\x04tool\x12:\n" +
	"\bapproval\x18\x05 \x01(\v2\x1c.mininaru.v1.ApprovalRequestH\x00R\bapproval\x12:\n" +
	"\tcompleted\x18\x06 \x01(\v2\x1a.mininaru.v1.ChatCompletedH\x00R\tcompleted\x121\n" +
	"\x06failed\x18\a \x01(\v2\x17.mininaru.v1.ChatFailedH\x00R\x06failed\x12=\n" +
	"\ftool_request\x18\b \x01(\v2\x18.mininaru.v1.ToolRequestH\x00R\vtoolRequestB\a\n" +
	"\x05event*\x99\x01\n" +
	"\fPairingState\x12\x1d\n" +
	"\x19PAIRING_STATE_UNSPECIFIED\x10\x00\x12\x19\n" +
	"\x15PAIRING_STATE_WAITING\x10\x01\x12\x1a\n" +
	"\x16PAIRING_STATE_APPROVED\x10\x02\x12\x18\n" +
	"\x14PAIRING_STATE_DENIED\x10\x03\x12\x19\n" +
	"\x15PAIRING_STATE_EXPIRED\x10\x04*\x82\x01\n" +
	"\x0eApprovalChoice\x12\x1f\n" +
	"\x1bAPPROVAL_CHOICE_UNSPECIFIED\x10\x00\x12\x18\n" +
	"\x14APPROVAL_CHOICE_DENY\x10\x01\x12\x18\n" +
	"\x14APPROVAL_CHOICE_ONCE\x10\x02\x12\x1b\n" +
	"\x17APPROVAL_CHOICE_SESSION\x10\x032\xa6\x01\n" +
	"\x0ePairingService\x12L\n" +
	"\x05Begin\x12 .mininaru.v1.BeginPairingRequest\x1a!.mininaru.v1.BeginPairingResponse\x12F\n" +
	"\x05Watch\x12 .mininaru.v1.WatchPairingRequest\x1a\x19.mininaru.v1.PairingEvent0\x012\xc9\x06\n" +
	"\x0fMininaruService\x12M\n" +
	"\n" +
	"ListAgents\x12\x1e.mininaru.v1.ListAgentsRequest\x1a\x1f.mininaru.v1.ListAgentsResponse\x12M\n" +
	"\n" +
	"ListSkills\x12\x1e.mininaru.v1.ListSkillsRequest\x1a\x1f.mininaru.v1.ListSkillsResponse\x12<\n" +
	"\bGetSkill\x12\x1c.mininaru.v1.GetSkillRequest\x1a\x12.mininaru.v1.Skill\x12S\n" +
	"\fListSessions\x12 .mininaru.v1.ListSessionsRequest\x1a!.mininaru.v1.ListSessionsResponse\x12H\n" +
	"\rCreateSession\x12!.mininaru.v1.CreateSessionRequest\x1a\x14.mininaru.v1.Session\x12H\n" +
	"\n" +
	"GetSession\x12\x1e.mininaru.v1.GetSessionRequest\x1a\x1a.mininaru.v1.SessionDetail\x12H\n" +
	"\rRenameSession\x12!.mininaru.v1.RenameSessionRequest\x1a\x14.mininaru.v1.Session\x12F\n" +
	"\rDeleteSession\x12!.mininaru.v1.DeleteSessionRequest\x1a\x12.mininaru.v1.Empty\x12<\n" +
	"\bGetUsage\x12\x1c.mininaru.v1.GetUsageRequest\x1a\x12.mininaru.v1.Usage\x12Y\n" +
	"\x0eCompactSession\x12\".mininaru.v1.CompactSessionRequest\x1a#.mininaru.v1.CompactSessionResponse\x12F\n" +
	"\x04Chat\x12\x1c.mininaru.v1.ChatClientEvent\x1a\x1c.mininaru.v1.ChatServerEvent(\x010\x01B=Z;github.com/devproje/mininaru/rpc/gen/mininaru/v1;mininaruv1b\x06proto3"

var (
	file_mininaru_v1_mininaru_proto_rawDescOnce sync.Once
	file_mininaru_v1_mininaru_proto_rawDescData []byte
)

func file_mininaru_v1_mininaru_proto_rawDescGZIP() []byte {
	file_mininaru_v1_mininaru_proto_rawDescOnce.Do(func() {
		file_mininaru_v1_mininaru_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_mininaru_v1_mininaru_proto_rawDesc), len(file_mininaru_v1_mininaru_proto_rawDesc)))
	})
	return file_mininaru_v1_mininaru_proto_rawDescData
}

var file_mininaru_v1_mininaru_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_mininaru_v1_mininaru_proto_msgTypes = make([]protoimpl.MessageInfo, 40)
var file_mininaru_v1_mininaru_proto_goTypes = []any{
	(PairingState)(0),              // 0: mininaru.v1.PairingState
	(ApprovalChoice)(0),            // 1: mininaru.v1.ApprovalChoice
	(*Empty)(nil),                  // 2: mininaru.v1.Empty
	(*BeginPairingRequest)(nil),    // 3: mininaru.v1.BeginPairingRequest
	(*BeginPairingResponse)(nil),   // 4: mininaru.v1.BeginPairingResponse
	(*WatchPairingRequest)(nil),    // 5: mininaru.v1.WatchPairingRequest
	(*PairingEvent)(nil),           // 6: mininaru.v1.PairingEvent
	(*Agent)(nil),                  // 7: mininaru.v1.Agent
	(*Session)(nil),                // 8: mininaru.v1.Session
	(*Message)(nil),                // 9: mininaru.v1.Message
	(*ToolCall)(nil),               // 10: mininaru.v1.ToolCall
	(*UsageLine)(nil),              // 11: mininaru.v1.UsageLine
	(*Usage)(nil),                  // 12: mininaru.v1.Usage
	(*ListAgentsRequest)(nil),      // 13: mininaru.v1.ListAgentsRequest
	(*ListAgentsResponse)(nil),     // 14: mininaru.v1.ListAgentsResponse
	(*Skill)(nil),                  // 15: mininaru.v1.Skill
	(*ListSkillsRequest)(nil),      // 16: mininaru.v1.ListSkillsRequest
	(*ListSkillsResponse)(nil),     // 17: mininaru.v1.ListSkillsResponse
	(*GetSkillRequest)(nil),        // 18: mininaru.v1.GetSkillRequest
	(*ListSessionsRequest)(nil),    // 19: mininaru.v1.ListSessionsRequest
	(*ListSessionsResponse)(nil),   // 20: mininaru.v1.ListSessionsResponse
	(*CreateSessionRequest)(nil),   // 21: mininaru.v1.CreateSessionRequest
	(*GetSessionRequest)(nil),      // 22: mininaru.v1.GetSessionRequest
	(*SessionDetail)(nil),          // 23: mininaru.v1.SessionDetail
	(*RenameSessionRequest)(nil),   // 24: mininaru.v1.RenameSessionRequest
	(*DeleteSessionRequest)(nil),   // 25: mininaru.v1.DeleteSessionRequest
	(*GetUsageRequest)(nil),        // 26: mininaru.v1.GetUsageRequest
	(*CompactSessionRequest)(nil),  // 27: mininaru.v1.CompactSessionRequest
	(*CompactSessionResponse)(nil), // 28: mininaru.v1.CompactSessionResponse
	(*ChatStart)(nil),              // 29: mininaru.v1.ChatStart
	(*ToolDefinition)(nil),         // 30: mininaru.v1.ToolDefinition
	(*ToolResult)(nil),             // 31: mininaru.v1.ToolResult
	(*ApprovalDecision)(nil),       // 32: mininaru.v1.ApprovalDecision
	(*ChatClientEvent)(nil),        // 33: mininaru.v1.ChatClientEvent
	(*ChatStarted)(nil),            // 34: mininaru.v1.ChatStarted
	(*TextDelta)(nil),              // 35: mininaru.v1.TextDelta
	(*ToolEvent)(nil),              // 36: mininaru.v1.ToolEvent
	(*ApprovalRequest)(nil),        // 37: mininaru.v1.ApprovalRequest
	(*ToolRequest)(nil),            // 38: mininaru.v1.ToolRequest
	(*ChatCompleted)(nil),          // 39: mininaru.v1.ChatCompleted
	(*ChatFailed)(nil),             // 40: mininaru.v1.ChatFailed
	(*ChatServerEvent)(nil),        // 41: mininaru.v1.ChatServerEvent
}
var file_mininaru_v1_mininaru_proto_depIdxs = []int32{
	0,  // 0: mininaru.v1.PairingEvent.state:type_name -> mininaru.v1.PairingState
	11, // 1: mininaru.v1.Usage.lines:type_name -> mininaru.v1.UsageLine
	7,  // 2: mininaru.v1.ListAgentsResponse.agents:type_name -> mininaru.v1.Agent
	15, // 3: mininaru.v1.ListSkillsResponse.skills:type_name -> mininaru.v1.Skill
	8,  // 4: mininaru.v1.ListSessionsResponse.sessions:type_name -> mininaru.v1.Session
	8,  // 5: mininaru.v1.SessionDetail.session:type_name -> mininaru.v1.Session
	7,  // 6: mininaru.v1.SessionDetail.agent:type_name -> mininaru.v1.Agent
	9,  // 7: mininaru.v1.SessionDetail.messages:type_name -> mininaru.v1.Message
	10, // 8: mininaru.v1.SessionDetail.tool_calls:type_name -> mininaru.v1.ToolCall
	30, // 9: mininaru.v1.ChatStart.tools:type_name -> mininaru.v1.ToolDefinition
	1,  // 10: mininaru.v1.ApprovalDecision.choice:type_name -> mininaru.v1.ApprovalChoice
	29, // 11: mininaru.v1.ChatClientEvent.start:type_name -> mininaru.v1.ChatStart
	32, // 12: mininaru.v1.ChatClientEvent.approval:type_name -> mininaru.v1.ApprovalDecision
	2,  // 13: mininaru.v1.ChatClientEvent.cancel:type_name -> mininaru.v1.Empty
	31, // 14: mininaru.v1.ChatClientEvent.tool_result:type_name -> mininaru.v1.ToolResult
	9,  // 15: mininaru.v1.ChatCompleted.message:type_name -> mininaru.v1.Message
	12, // 16: mininaru.v1.ChatCompleted.usage:type_name -> mininaru.v1.Usage
	34, // 17: mininaru.v1.ChatServerEvent.started:type_name -> mininaru.v1.ChatStarted
	35, // 18: mininaru.v1.ChatServerEvent.content:type_name -> mininaru.v1.TextDelta
	35, // 19: mininaru.v1.ChatServerEvent.reasoning:type_name -> mininaru.v1.TextDelta
	36, // 20: mininaru.v1.ChatServerEvent.tool:type_name -> mininaru.v1.ToolEvent
	37, // 21: mininaru.v1.ChatServerEvent.approval:type_name -> mininaru.v1.ApprovalRequest
	39, // 22: mininaru.v1.ChatServerEvent.completed:type_name -> mininaru.v1.ChatCompleted
	40, // 23: mininaru.v1.ChatServerEvent.failed:type_name -> mininaru.v1.ChatFailed
	38, // 24: mininaru.v1.ChatServerEvent.tool_request:type_name -> mininaru.v1.ToolRequest
	3,  // 25: mininaru.v1.PairingService.Begin:input_type -> mininaru.v1.BeginPairingRequest
	5,  // 26: mininaru.v1.PairingService.Watch:input_type -> mininaru.v1.WatchPairingRequest
	13, // 27: mininaru.v1.MininaruService.ListAgents:input_type -> mininaru.v1.ListAgentsRequest
	16, // 28: mininaru.v1.MininaruService.ListSkills:input_type -> mininaru.v1.ListSkillsRequest
	18, // 29: mininaru.v1.MininaruService.GetSkill:input_type -> mininaru.v1.GetSkillRequest
	19, // 30: mininaru.v1.MininaruService.ListSessions:input_type -> mininaru.v1.ListSessionsRequest
	21, // 31: mininaru.v1.MininaruService.CreateSession:input_type -> mininaru.v1.CreateSessionRequest
	22, // 32: mininaru.v1.MininaruService.GetSession:input_type -> mininaru.v1.GetSessionRequest
	24, // 33: mininaru.v1.MininaruService.RenameSession:input_type -> mininaru.v1.RenameSessionRequest
	25, // 34: mininaru.v1.MininaruService.DeleteSession:input_type -> mininaru.v1.DeleteSessionRequest
	26, // 35: mininaru.v1.MininaruService.GetUsage:input_type -> mininaru.v1.GetUsageRequest
	27, // 36: mininaru.v1.MininaruService.CompactSession:input_type -> mininaru.v1.CompactSessionRequest
	33, // 37: mininaru.v1.MininaruService.Chat:input_type -> mininaru.v1.ChatClientEvent
	4,  // 38: mininaru.v1.PairingService.Begin:output_type -> mininaru.v1.BeginPairingResponse
	6,  // 39: mininaru.v1.PairingService.Watch:output_type -> mininaru.v1.PairingEvent
	14, // 40: mininaru.v1.MininaruService.ListAgents:output_type -> mininaru.v1.ListAgentsResponse
	17, // 41: mininaru.v1.MininaruService.ListSkills:output_type -> mininaru.v1.ListSkillsResponse
	15, // 42: mininaru.v1.MininaruService.GetSkill:output_type -> mininaru.v1.Skill
	20, // 43: mininaru.v1.MininaruService.ListSessions:output_type -> mininaru.v1.ListSessionsResponse
	8,  // 44: mininaru.v1.MininaruService.CreateSession:output_type -> mininaru.v1.Session
	23, // 45: mininaru.v1.MininaruService.GetSession:output_type -> mininaru.v1.SessionDetail
	8,  // 46: mininaru.v1.MininaruService.RenameSession:output_type -> mininaru.v1.Session
	2,  // 47: mininaru.v1.MininaruService.DeleteSession:output_type -> mininaru.v1.Empty
	12, // 48: mininaru.v1.MininaruService.GetUsage:output_type -> mininaru.v1.Usage
	28, // 49: mininaru.v1.MininaruService.CompactSession:output_type -> mininaru.v1.CompactSessionResponse
	41, // 50: mininaru.v1.MininaruService.Chat:output_type -> mininaru.v1.ChatServerEvent
	38, // [38:51] is the sub-list for method output_type
	25, // [25:38] is the sub-list for method input_type
	25, // [25:25] is the sub-list for extension type_name
	25, // [25:25] is the sub-list for extension extendee
	0,  // [0:25] is the sub-list for field type_name
}

func init() { file_mininaru_v1_mininaru_proto_init() }
func file_mininaru_v1_mininaru_proto_init() {
	var ()
	if File_mininaru_v1_mininaru_proto != nil {
		return
	}
	file_mininaru_v1_mininaru_proto_msgTypes[31].OneofWrappers = []any{
		(*ChatClientEvent_Start)(nil),
		(*ChatClientEvent_Approval)(nil),
		(*ChatClientEvent_Cancel)(nil),
		(*ChatClientEvent_ToolResult)(nil),
	}
	file_mininaru_v1_mininaru_proto_msgTypes[39].OneofWrappers = []any{
		(*ChatServerEvent_Started)(nil),
		(*ChatServerEvent_Content)(nil),
		(*ChatServerEvent_Reasoning)(nil),
		(*ChatServerEvent_Tool)(nil),
		(*ChatServerEvent_Approval)(nil),
		(*ChatServerEvent_Completed)(nil),
		(*ChatServerEvent_Failed)(nil),
		(*ChatServerEvent_ToolRequest)(nil),
	}
	type x struct{}

	File_mininaru_v1_mininaru_proto = protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_mininaru_v1_mininaru_proto_rawDesc), len(file_mininaru_v1_mininaru_proto_rawDesc)),
			NumEnums:      2,
			NumMessages:   40,
			NumExtensions: 0,
			NumServices:   2,
		},
		GoTypes:           file_mininaru_v1_mininaru_proto_goTypes,
		DependencyIndexes: file_mininaru_v1_mininaru_proto_depIdxs,
		EnumInfos:         file_mininaru_v1_mininaru_proto_enumTypes,
		MessageInfos:      file_mininaru_v1_mininaru_proto_msgTypes,
	}.Build().File

	file_mininaru_v1_mininaru_proto_goTypes = nil
	file_mininaru_v1_mininaru_proto_depIdxs = nil
}
