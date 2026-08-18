// SPDX-FileCopyrightText: 2026 Wonhyeok Kim (Project_IO)
// SPDX-License-Identifier: GPL-3.0-or-later

package mininaruv1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const _ = grpc.SupportPackageIsVersion9

const (
	PairingService_Begin_FullMethodName = "/mininaru.v1.PairingService/Begin"
	PairingService_Watch_FullMethodName = "/mininaru.v1.PairingService/Watch"
)

type PairingServiceClient interface {
	Begin(ctx context.Context, in *BeginPairingRequest, opts ...grpc.CallOption) (*BeginPairingResponse, error)
	Watch(ctx context.Context, in *WatchPairingRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[PairingEvent], error)
}

type pairingServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPairingServiceClient(cc grpc.ClientConnInterface) PairingServiceClient {
	return &pairingServiceClient{cc}
}

func (c *pairingServiceClient) Begin(ctx context.Context, in *BeginPairingRequest, opts ...grpc.CallOption) (*BeginPairingResponse, error) {
	var (
		cOpts []grpc.CallOption
		out   *BeginPairingResponse
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(BeginPairingResponse)
	err = c.cc.Invoke(ctx, PairingService_Begin_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pairingServiceClient) Watch(ctx context.Context, in *WatchPairingRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[PairingEvent], error) {
	var (
		cOpts  []grpc.CallOption
		stream grpc.ClientStream
		x      *grpc.GenericClientStream[WatchPairingRequest, PairingEvent]
		err    error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err = c.cc.NewStream(ctx, &PairingService_ServiceDesc.Streams[0], PairingService_Watch_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x = &grpc.GenericClientStream[WatchPairingRequest, PairingEvent]{ClientStream: stream}
	if err = x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err = x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type PairingService_WatchClient = grpc.ServerStreamingClient[PairingEvent]

type PairingServiceServer interface {
	Begin(context.Context, *BeginPairingRequest) (*BeginPairingResponse, error)
	Watch(*WatchPairingRequest, grpc.ServerStreamingServer[PairingEvent]) error
	mustEmbedUnimplementedPairingServiceServer()
}

type UnimplementedPairingServiceServer struct{}

func (UnimplementedPairingServiceServer) Begin(context.Context, *BeginPairingRequest) (*BeginPairingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Begin not implemented")
}
func (UnimplementedPairingServiceServer) Watch(*WatchPairingRequest, grpc.ServerStreamingServer[PairingEvent]) error {
	return status.Error(codes.Unimplemented, "method Watch not implemented")
}
func (UnimplementedPairingServiceServer) mustEmbedUnimplementedPairingServiceServer() {}
func (UnimplementedPairingServiceServer) testEmbeddedByValue()                        {}

type UnsafePairingServiceServer interface {
	mustEmbedUnimplementedPairingServiceServer()
}

func RegisterPairingServiceServer(s grpc.ServiceRegistrar, srv PairingServiceServer) {
	var (
		t  interface{ testEmbeddedByValue() }
		ok bool
	)

	if t, ok = srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&PairingService_ServiceDesc, srv)
}

func _PairingService_Begin_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *BeginPairingRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(BeginPairingRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PairingServiceServer).Begin(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PairingService_Begin_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PairingServiceServer).Begin(ctx, req.(*BeginPairingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PairingService_Watch_Handler(srv interface{}, stream grpc.ServerStream) error {
	var (
		m   *WatchPairingRequest
		err error
	)

	m = new(WatchPairingRequest)
	if err = stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(PairingServiceServer).Watch(m, &grpc.GenericServerStream[WatchPairingRequest, PairingEvent]{ServerStream: stream})
}

type PairingService_WatchServer = grpc.ServerStreamingServer[PairingEvent]

var PairingService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "mininaru.v1.PairingService",
	HandlerType: (*PairingServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Begin",
			Handler:    _PairingService_Begin_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Watch",
			Handler:       _PairingService_Watch_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "mininaru/v1/mininaru.proto",
}

const (
	MininaruService_ListAgents_FullMethodName     = "/mininaru.v1.MininaruService/ListAgents"
	MininaruService_ListSessions_FullMethodName   = "/mininaru.v1.MininaruService/ListSessions"
	MininaruService_CreateSession_FullMethodName  = "/mininaru.v1.MininaruService/CreateSession"
	MininaruService_GetSession_FullMethodName     = "/mininaru.v1.MininaruService/GetSession"
	MininaruService_RenameSession_FullMethodName  = "/mininaru.v1.MininaruService/RenameSession"
	MininaruService_DeleteSession_FullMethodName  = "/mininaru.v1.MininaruService/DeleteSession"
	MininaruService_GetUsage_FullMethodName       = "/mininaru.v1.MininaruService/GetUsage"
	MininaruService_CompactSession_FullMethodName = "/mininaru.v1.MininaruService/CompactSession"
	MininaruService_Chat_FullMethodName           = "/mininaru.v1.MininaruService/Chat"
)

type MininaruServiceClient interface {
	ListAgents(ctx context.Context, in *ListAgentsRequest, opts ...grpc.CallOption) (*ListAgentsResponse, error)
	ListSessions(ctx context.Context, in *ListSessionsRequest, opts ...grpc.CallOption) (*ListSessionsResponse, error)
	CreateSession(ctx context.Context, in *CreateSessionRequest, opts ...grpc.CallOption) (*Session, error)
	GetSession(ctx context.Context, in *GetSessionRequest, opts ...grpc.CallOption) (*SessionDetail, error)
	RenameSession(ctx context.Context, in *RenameSessionRequest, opts ...grpc.CallOption) (*Session, error)
	DeleteSession(ctx context.Context, in *DeleteSessionRequest, opts ...grpc.CallOption) (*Empty, error)
	GetUsage(ctx context.Context, in *GetUsageRequest, opts ...grpc.CallOption) (*Usage, error)
	CompactSession(ctx context.Context, in *CompactSessionRequest, opts ...grpc.CallOption) (*CompactSessionResponse, error)
	Chat(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ChatClientEvent, ChatServerEvent], error)
}

type mininaruServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewMininaruServiceClient(cc grpc.ClientConnInterface) MininaruServiceClient {
	return &mininaruServiceClient{cc}
}

func (c *mininaruServiceClient) ListAgents(ctx context.Context, in *ListAgentsRequest, opts ...grpc.CallOption) (*ListAgentsResponse, error) {
	var (
		cOpts []grpc.CallOption
		out   *ListAgentsResponse
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(ListAgentsResponse)
	err = c.cc.Invoke(ctx, MininaruService_ListAgents_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mininaruServiceClient) ListSessions(ctx context.Context, in *ListSessionsRequest, opts ...grpc.CallOption) (*ListSessionsResponse, error) {
	var (
		cOpts []grpc.CallOption
		out   *ListSessionsResponse
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(ListSessionsResponse)
	err = c.cc.Invoke(ctx, MininaruService_ListSessions_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mininaruServiceClient) CreateSession(ctx context.Context, in *CreateSessionRequest, opts ...grpc.CallOption) (*Session, error) {
	var (
		cOpts []grpc.CallOption
		out   *Session
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(Session)
	err = c.cc.Invoke(ctx, MininaruService_CreateSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mininaruServiceClient) GetSession(ctx context.Context, in *GetSessionRequest, opts ...grpc.CallOption) (*SessionDetail, error) {
	var (
		cOpts []grpc.CallOption
		out   *SessionDetail
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(SessionDetail)
	err = c.cc.Invoke(ctx, MininaruService_GetSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mininaruServiceClient) RenameSession(ctx context.Context, in *RenameSessionRequest, opts ...grpc.CallOption) (*Session, error) {
	var (
		cOpts []grpc.CallOption
		out   *Session
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(Session)
	err = c.cc.Invoke(ctx, MininaruService_RenameSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mininaruServiceClient) DeleteSession(ctx context.Context, in *DeleteSessionRequest, opts ...grpc.CallOption) (*Empty, error) {
	var (
		cOpts []grpc.CallOption
		out   *Empty
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(Empty)
	err = c.cc.Invoke(ctx, MininaruService_DeleteSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mininaruServiceClient) GetUsage(ctx context.Context, in *GetUsageRequest, opts ...grpc.CallOption) (*Usage, error) {
	var (
		cOpts []grpc.CallOption
		out   *Usage
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(Usage)
	err = c.cc.Invoke(ctx, MininaruService_GetUsage_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mininaruServiceClient) CompactSession(ctx context.Context, in *CompactSessionRequest, opts ...grpc.CallOption) (*CompactSessionResponse, error) {
	var (
		cOpts []grpc.CallOption
		out   *CompactSessionResponse
		err   error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out = new(CompactSessionResponse)
	err = c.cc.Invoke(ctx, MininaruService_CompactSession_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mininaruServiceClient) Chat(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[ChatClientEvent, ChatServerEvent], error) {
	var (
		cOpts  []grpc.CallOption
		stream grpc.ClientStream
		x      *grpc.GenericClientStream[ChatClientEvent, ChatServerEvent]
		err    error
	)

	cOpts = append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err = c.cc.NewStream(ctx, &MininaruService_ServiceDesc.Streams[0], MininaruService_Chat_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x = &grpc.GenericClientStream[ChatClientEvent, ChatServerEvent]{ClientStream: stream}
	return x, nil
}

type MininaruService_ChatClient = grpc.BidiStreamingClient[ChatClientEvent, ChatServerEvent]

type MininaruServiceServer interface {
	ListAgents(context.Context, *ListAgentsRequest) (*ListAgentsResponse, error)
	ListSessions(context.Context, *ListSessionsRequest) (*ListSessionsResponse, error)
	CreateSession(context.Context, *CreateSessionRequest) (*Session, error)
	GetSession(context.Context, *GetSessionRequest) (*SessionDetail, error)
	RenameSession(context.Context, *RenameSessionRequest) (*Session, error)
	DeleteSession(context.Context, *DeleteSessionRequest) (*Empty, error)
	GetUsage(context.Context, *GetUsageRequest) (*Usage, error)
	CompactSession(context.Context, *CompactSessionRequest) (*CompactSessionResponse, error)
	Chat(grpc.BidiStreamingServer[ChatClientEvent, ChatServerEvent]) error
	mustEmbedUnimplementedMininaruServiceServer()
}

type UnimplementedMininaruServiceServer struct{}

func (UnimplementedMininaruServiceServer) ListAgents(context.Context, *ListAgentsRequest) (*ListAgentsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListAgents not implemented")
}
func (UnimplementedMininaruServiceServer) ListSessions(context.Context, *ListSessionsRequest) (*ListSessionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListSessions not implemented")
}
func (UnimplementedMininaruServiceServer) CreateSession(context.Context, *CreateSessionRequest) (*Session, error) {
	return nil, status.Error(codes.Unimplemented, "method CreateSession not implemented")
}
func (UnimplementedMininaruServiceServer) GetSession(context.Context, *GetSessionRequest) (*SessionDetail, error) {
	return nil, status.Error(codes.Unimplemented, "method GetSession not implemented")
}
func (UnimplementedMininaruServiceServer) RenameSession(context.Context, *RenameSessionRequest) (*Session, error) {
	return nil, status.Error(codes.Unimplemented, "method RenameSession not implemented")
}
func (UnimplementedMininaruServiceServer) DeleteSession(context.Context, *DeleteSessionRequest) (*Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method DeleteSession not implemented")
}
func (UnimplementedMininaruServiceServer) GetUsage(context.Context, *GetUsageRequest) (*Usage, error) {
	return nil, status.Error(codes.Unimplemented, "method GetUsage not implemented")
}
func (UnimplementedMininaruServiceServer) CompactSession(context.Context, *CompactSessionRequest) (*CompactSessionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method CompactSession not implemented")
}
func (UnimplementedMininaruServiceServer) Chat(grpc.BidiStreamingServer[ChatClientEvent, ChatServerEvent]) error {
	return status.Error(codes.Unimplemented, "method Chat not implemented")
}
func (UnimplementedMininaruServiceServer) mustEmbedUnimplementedMininaruServiceServer() {}
func (UnimplementedMininaruServiceServer) testEmbeddedByValue()                         {}

type UnsafeMininaruServiceServer interface {
	mustEmbedUnimplementedMininaruServiceServer()
}

func RegisterMininaruServiceServer(s grpc.ServiceRegistrar, srv MininaruServiceServer) {
	var (
		t  interface{ testEmbeddedByValue() }
		ok bool
	)

	if t, ok = srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&MininaruService_ServiceDesc, srv)
}

func _MininaruService_ListAgents_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *ListAgentsRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(ListAgentsRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MininaruServiceServer).ListAgents(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MininaruService_ListAgents_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MininaruServiceServer).ListAgents(ctx, req.(*ListAgentsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MininaruService_ListSessions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *ListSessionsRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(ListSessionsRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MininaruServiceServer).ListSessions(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MininaruService_ListSessions_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MininaruServiceServer).ListSessions(ctx, req.(*ListSessionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MininaruService_CreateSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *CreateSessionRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(CreateSessionRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MininaruServiceServer).CreateSession(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MininaruService_CreateSession_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MininaruServiceServer).CreateSession(ctx, req.(*CreateSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MininaruService_GetSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *GetSessionRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(GetSessionRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MininaruServiceServer).GetSession(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MininaruService_GetSession_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MininaruServiceServer).GetSession(ctx, req.(*GetSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MininaruService_RenameSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *RenameSessionRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(RenameSessionRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MininaruServiceServer).RenameSession(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MininaruService_RenameSession_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MininaruServiceServer).RenameSession(ctx, req.(*RenameSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MininaruService_DeleteSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *DeleteSessionRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(DeleteSessionRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MininaruServiceServer).DeleteSession(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MininaruService_DeleteSession_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MininaruServiceServer).DeleteSession(ctx, req.(*DeleteSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MininaruService_GetUsage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *GetUsageRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(GetUsageRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MininaruServiceServer).GetUsage(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MininaruService_GetUsage_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MininaruServiceServer).GetUsage(ctx, req.(*GetUsageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MininaruService_CompactSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var (
		in      *CompactSessionRequest
		info    *grpc.UnaryServerInfo
		handler func(ctx context.Context, req interface{}) (interface{}, error)
		err     error
	)

	in = new(CompactSessionRequest)
	if err = dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MininaruServiceServer).CompactSession(ctx, in)
	}
	info = &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: MininaruService_CompactSession_FullMethodName,
	}
	handler = func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MininaruServiceServer).CompactSession(ctx, req.(*CompactSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _MininaruService_Chat_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(MininaruServiceServer).Chat(&grpc.GenericServerStream[ChatClientEvent, ChatServerEvent]{ServerStream: stream})
}

type MininaruService_ChatServer = grpc.BidiStreamingServer[ChatClientEvent, ChatServerEvent]

var MininaruService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "mininaru.v1.MininaruService",
	HandlerType: (*MininaruServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListAgents",
			Handler:    _MininaruService_ListAgents_Handler,
		},
		{
			MethodName: "ListSessions",
			Handler:    _MininaruService_ListSessions_Handler,
		},
		{
			MethodName: "CreateSession",
			Handler:    _MininaruService_CreateSession_Handler,
		},
		{
			MethodName: "GetSession",
			Handler:    _MininaruService_GetSession_Handler,
		},
		{
			MethodName: "RenameSession",
			Handler:    _MininaruService_RenameSession_Handler,
		},
		{
			MethodName: "DeleteSession",
			Handler:    _MininaruService_DeleteSession_Handler,
		},
		{
			MethodName: "GetUsage",
			Handler:    _MininaruService_GetUsage_Handler,
		},
		{
			MethodName: "CompactSession",
			Handler:    _MininaruService_CompactSession_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Chat",
			Handler:       _MininaruService_Chat_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "mininaru/v1/mininaru.proto",
}
