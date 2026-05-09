// Code generated manually (protoc not available). DO NOT EDIT.
// Source: proto/project.proto

package proto

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProjectServiceClient is the client API for ProjectService.
type ProjectServiceClient interface {
	GetProject(ctx context.Context, in *GetProjectRequest, opts ...grpc.CallOption) (*ProjectInfo, error)
	ListProjects(ctx context.Context, in *ListProjectsRequest, opts ...grpc.CallOption) (*ListProjectsResponse, error)
}

type projectServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewProjectServiceClient(cc grpc.ClientConnInterface) ProjectServiceClient {
	return &projectServiceClient{cc}
}

func (c *projectServiceClient) GetProject(ctx context.Context, in *GetProjectRequest, opts ...grpc.CallOption) (*ProjectInfo, error) {
	out := new(ProjectInfo)
	err := c.cc.Invoke(ctx, "/project.v1.ProjectService/GetProject", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *projectServiceClient) ListProjects(ctx context.Context, in *ListProjectsRequest, opts ...grpc.CallOption) (*ListProjectsResponse, error) {
	out := new(ListProjectsResponse)
	err := c.cc.Invoke(ctx, "/project.v1.ProjectService/ListProjects", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ProjectServiceServer is the server API for ProjectService.
type ProjectServiceServer interface {
	GetProject(context.Context, *GetProjectRequest) (*ProjectInfo, error)
	ListProjects(context.Context, *ListProjectsRequest) (*ListProjectsResponse, error)
	mustEmbedUnimplementedProjectServiceServer()
}

// UnimplementedProjectServiceServer must be embedded to have forward-compatible implementations.
type UnimplementedProjectServiceServer struct{}

func (UnimplementedProjectServiceServer) GetProject(context.Context, *GetProjectRequest) (*ProjectInfo, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetProject not implemented")
}

func (UnimplementedProjectServiceServer) ListProjects(context.Context, *ListProjectsRequest) (*ListProjectsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListProjects not implemented")
}

func (UnimplementedProjectServiceServer) mustEmbedUnimplementedProjectServiceServer() {}

// RegisterProjectServiceServer registers the server implementation.
func RegisterProjectServiceServer(s grpc.ServiceRegistrar, srv ProjectServiceServer) {
	s.RegisterService(&ProjectService_ServiceDesc, srv)
}

func _ProjectService_GetProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectServiceServer).GetProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/project.v1.ProjectService/GetProject",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectServiceServer).GetProject(ctx, req.(*GetProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProjectService_ListProjects_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListProjectsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProjectServiceServer).ListProjects(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/project.v1.ProjectService/ListProjects",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProjectServiceServer).ListProjects(ctx, req.(*ListProjectsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ProjectService_ServiceDesc is the grpc.ServiceDesc for ProjectService.
var ProjectService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "project.v1.ProjectService",
	HandlerType: (*ProjectServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetProject",
			Handler:    _ProjectService_GetProject_Handler,
		},
		{
			MethodName: "ListProjects",
			Handler:    _ProjectService_ListProjects_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "project.proto",
}
