package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	userv1 "userfrontendservice/gen/user/v1"
	"userfrontendservice/gen/user/v1/userv1connect"

	"connectrpc.com/connect"

	"github.com/nats-io/nats.go"
	"github.com/rs/cors"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	_ "github.com/joho/godotenv/autoload"
)

type UserServer struct {
	nc *nats.Conn
}

// CreateUser implements userv1connect.UserFrontendServiceHandler.
func (p *UserServer) CreateUser(ctx context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error) {
	user := req.Msg.GetUser()
	data, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}

	msg, err := p.nc.Request("CreateUser", data, nats.DefaultTimeout)
	if err != nil {
		return nil, err
	}

	var createdUser userv1.User
	err = json.Unmarshal(msg.Data, &createdUser)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&userv1.CreateUserResponse{Id: createdUser.Id}), nil
}

// DeleteUser implements userv1connect.UserFrontendServiceHandler.
func (p *UserServer) DeleteUser(ctx context.Context, req *connect.Request[userv1.DeleteUserRequest]) (*connect.Response[userv1.DeleteUserResponse], error) {
	id := req.Msg.GetId()
	log.Println(id)
	_, err := p.nc.Request("DeleteUser", []byte(id), nats.DefaultTimeout)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&userv1.DeleteUserResponse{}), nil
}

// ReadAllUsers implements userv1connect.UserFrontendServiceHandler.
func (p *UserServer) ReadAllUsers(ctx context.Context, req *connect.Request[userv1.ReadAllUsersRequest]) (*connect.Response[userv1.ReadAllUsersResponse], error) {
	msg, err := p.nc.Request("ReadAllUsers", []byte{}, nats.DefaultTimeout)
	if err != nil {
		return nil, err
	}

	var users []*userv1.User
	err = json.Unmarshal(msg.Data, &users)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&userv1.ReadAllUsersResponse{Users: users}), nil
}

// ReadUser implements userv1connect.UserFrontendServiceHandler.
func (p *UserServer) ReadUser(ctx context.Context, req *connect.Request[userv1.ReadUserRequest]) (*connect.Response[userv1.ReadUserResponse], error) {
	id := req.Msg.GetId()
	msg, err := p.nc.Request("ReadUser", []byte(id), nats.DefaultTimeout)
	if err != nil {
		return nil, err
	}

	var user userv1.User
	err = json.Unmarshal(msg.Data, &user)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&userv1.ReadUserResponse{User: &user}), nil
}

// UpdateUser implements userv1connect.UserFrontendServiceHandler.
func (p *UserServer) UpdateUser(ctx context.Context, req *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error) {
	user := req.Msg.GetUser()
	data, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}

	msg, err := p.nc.Request("UpdateUser", data, nats.DefaultTimeout)
	if err != nil {
		return nil, err
	}

	var updatedUser userv1.User
	err = json.Unmarshal(msg.Data, &updatedUser)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&userv1.UpdateUserResponse{}), nil
}

func main() {
	natsURL := os.Getenv("NATS_URL")

	// natsURL := "nats://nats.loopabord.svc.cluster.local:4222"
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Println(err)
	}
	defer nc.Close()

	server := &UserServer{nc: nc}
	mux := http.NewServeMux()
	path, handler := userv1connect.NewUserFrontendServiceHandler(server)
	mux.Handle(path, handler)

	// CORS middleware
	corsHandler := cors.AllowAll().Handler(mux)

	// Start server
	http.ListenAndServe("0.0.0.0:8081", h2c.NewHandler(corsHandler, &http2.Server{}))
}
