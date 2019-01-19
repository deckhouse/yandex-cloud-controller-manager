// Code generated by sdkgen. DO NOT EDIT.

// nolint
package clickhouse

import (
	"context"

	"google.golang.org/grpc"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/mdb/clickhouse/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/operation"
)

// UserServiceClient is a clickhouse.UserServiceClient with
// lazy GRPC connection initialization.
type UserServiceClient struct {
	getConn func(ctx context.Context) (*grpc.ClientConn, error)
}

var _ clickhouse.UserServiceClient = &UserServiceClient{}

// Create implements clickhouse.UserServiceClient
func (c *UserServiceClient) Create(ctx context.Context, in *clickhouse.CreateUserRequest, opts ...grpc.CallOption) (*operation.Operation, error) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	return clickhouse.NewUserServiceClient(conn).Create(ctx, in, opts...)
}

// Delete implements clickhouse.UserServiceClient
func (c *UserServiceClient) Delete(ctx context.Context, in *clickhouse.DeleteUserRequest, opts ...grpc.CallOption) (*operation.Operation, error) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	return clickhouse.NewUserServiceClient(conn).Delete(ctx, in, opts...)
}

// Get implements clickhouse.UserServiceClient
func (c *UserServiceClient) Get(ctx context.Context, in *clickhouse.GetUserRequest, opts ...grpc.CallOption) (*clickhouse.User, error) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	return clickhouse.NewUserServiceClient(conn).Get(ctx, in, opts...)
}

// GrantPermission implements clickhouse.UserServiceClient
func (c *UserServiceClient) GrantPermission(ctx context.Context, in *clickhouse.GrantUserPermissionRequest, opts ...grpc.CallOption) (*operation.Operation, error) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	return clickhouse.NewUserServiceClient(conn).GrantPermission(ctx, in, opts...)
}

// List implements clickhouse.UserServiceClient
func (c *UserServiceClient) List(ctx context.Context, in *clickhouse.ListUsersRequest, opts ...grpc.CallOption) (*clickhouse.ListUsersResponse, error) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	return clickhouse.NewUserServiceClient(conn).List(ctx, in, opts...)
}

// RevokePermission implements clickhouse.UserServiceClient
func (c *UserServiceClient) RevokePermission(ctx context.Context, in *clickhouse.RevokeUserPermissionRequest, opts ...grpc.CallOption) (*operation.Operation, error) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	return clickhouse.NewUserServiceClient(conn).RevokePermission(ctx, in, opts...)
}

// Update implements clickhouse.UserServiceClient
func (c *UserServiceClient) Update(ctx context.Context, in *clickhouse.UpdateUserRequest, opts ...grpc.CallOption) (*operation.Operation, error) {
	conn, err := c.getConn(ctx)
	if err != nil {
		return nil, err
	}
	return clickhouse.NewUserServiceClient(conn).Update(ctx, in, opts...)
}
