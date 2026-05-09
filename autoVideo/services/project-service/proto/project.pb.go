// Code generated manually (protoc not available). DO NOT EDIT.
// Source: proto/project.proto

package proto

import "fmt"

// GetProjectRequest is the request for GetProject RPC.
type GetProjectRequest struct {
	ProjectId uint64 `protobuf:"varint,1,opt,name=project_id,json=projectId,proto3" json:"project_id,omitempty"`
	UserId    uint64 `protobuf:"varint,2,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
}

func (m *GetProjectRequest) Reset()         { *m = GetProjectRequest{} }
func (m *GetProjectRequest) String() string { return fmt.Sprintf("project_id:%d user_id:%d", m.ProjectId, m.UserId) }
func (m *GetProjectRequest) ProtoMessage()  {}

// ProjectInfo is the project representation returned by gRPC.
type ProjectInfo struct {
	Id     uint64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	UserId uint64 `protobuf:"varint,2,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Title  string `protobuf:"bytes,3,opt,name=title,proto3" json:"title,omitempty"`
	Status string `protobuf:"bytes,4,opt,name=status,proto3" json:"status,omitempty"`
}

func (m *ProjectInfo) Reset()         { *m = ProjectInfo{} }
func (m *ProjectInfo) String() string { return fmt.Sprintf("id:%d title:%s", m.Id, m.Title) }
func (m *ProjectInfo) ProtoMessage()  {}

// ListProjectsRequest is the request for ListProjects RPC.
type ListProjectsRequest struct {
	UserId   uint64 `protobuf:"varint,1,opt,name=user_id,json=userId,proto3" json:"user_id,omitempty"`
	Page     int32  `protobuf:"varint,2,opt,name=page,proto3" json:"page,omitempty"`
	PageSize int32  `protobuf:"varint,3,opt,name=page_size,json=pageSize,proto3" json:"page_size,omitempty"`
}

func (m *ListProjectsRequest) Reset()         { *m = ListProjectsRequest{} }
func (m *ListProjectsRequest) String() string { return fmt.Sprintf("user_id:%d", m.UserId) }
func (m *ListProjectsRequest) ProtoMessage()  {}

// ListProjectsResponse is the response for ListProjects RPC.
type ListProjectsResponse struct {
	Projects []*ProjectInfo `protobuf:"bytes,1,rep,name=projects,proto3" json:"projects,omitempty"`
	Total    int64          `protobuf:"varint,2,opt,name=total,proto3" json:"total,omitempty"`
}

func (m *ListProjectsResponse) Reset()         { *m = ListProjectsResponse{} }
func (m *ListProjectsResponse) String() string { return fmt.Sprintf("total:%d", m.Total) }
func (m *ListProjectsResponse) ProtoMessage()  {}
