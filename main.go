package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	userv1 "userfrontendservice/gen/user/v1"
	"userfrontendservice/gen/user/v1/userv1connect"

	"connectrpc.com/connect"

	"github.com/nats-io/nats.go"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/form3tech-oss/jwt-go"

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

func (p *UserServer) UpdateUser(ctx context.Context, req *connect.Request[userv1.UpdateUserRequest]) (*connect.Response[userv1.UpdateUserResponse], error) {
	updatedUser := req.Msg.GetUser()

	// Fetch original user data from UserService
	msg, err := p.nc.Request("ReadUser", []byte(updatedUser.Id), nats.DefaultTimeout)
	if err != nil {
		return nil, err
	}

	// Accessing 'sub' from context
	token, ok := ctx.Value("user").(*jwt.Token)
	if !ok {
		return nil, errors.New("unable to retrieve JWT token from context")
	}
	claims := token.Claims.(jwt.MapClaims)
	sub := claims["sub"].(string)

	if sub != updatedUser.Id {
		return nil, errors.New("unauthorized")
	}

	var originalUserData []byte
	originalUserData = append(originalUserData, msg.Data...)

	// Marshal updated user data for NATS message
	data, err := json.Marshal(updatedUser)
	if err != nil {
		return nil, err
	}

	// Send UpdateUser message to UserService
	msg, err = p.nc.Request("UpdateUser", data, nats.DefaultTimeout)
	if err != nil {
		// Handle NATS request error
		return nil, err
	}

	var user userv1.User
	err = json.Unmarshal(msg.Data, &user)
	if err != nil {
		return nil, err
	}

	// If UserService successfully updates the user, publish UserUpdated event
	if _, err := p.nc.Request("UserUpdated", data, nats.DefaultTimeout); err != nil {
		// Handle publish UserUpdated event error
		// Attempt to revert by sending UpdateUser request with original user data
		if _, err := p.nc.Request("UpdateUser", originalUserData, nats.DefaultTimeout); err != nil {
			// Handle revert UpdateUser error
			return nil, err
		}

		return nil, fmt.Errorf("failed to publish UserUpdated event and revert update")
	}

	// Return success response
	return connect.NewResponse(&userv1.UpdateUserResponse{}), nil
}

func UpdateUserName(userID string, name string) {
	// Auth0 Management API endpoint for updating a user
	apiEndpoint := fmt.Sprintf("https://YOUR_AUTH0_DOMAIN/api/v2/users/%s", userID)

	// Auth0 API token with appropriate permissions (should be stored securely in production)
	apiToken := "YOUR_AUTH0_API_TOKEN"

	// Prepare the updated nickname
	newNickname := "new_nickname"

	// Create JSON payload for the request body
	updatePayload := newNickname

	payloadBytes, err := json.Marshal(updatePayload)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	// Create HTTP PUT request
	req, err := http.NewRequest("PATCH", apiEndpoint, bytes.NewBuffer(payloadBytes))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	// Send HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Failed to update nickname:", resp.Status)
		return
	}

	fmt.Println("Nickname updated successfully!")
}

func main() {
	natsURL := os.Getenv("NATS_URL")
	log.Println(natsURL)

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Println(err)
	}
	defer nc.Close()

	server := &UserServer{nc: nc}
	mux := http.NewServeMux()
	path, handler := userv1connect.NewUserFrontendServiceHandler(server)
	mux.Handle(path, handler)

	// Handle CORS headers
	corsWrapper := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow requests from any origin
			w.Header().Set("Access-Control-Allow-Origin", "*")
			// Allow specific headers
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, connect-protocol-version")
			// Allow specific methods
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			// Call the actual handler
			h.ServeHTTP(w, r)
		})
	}

	// Wrap the CORS middleware around your mux
	corsHandler := corsWrapper(mux)

	// Create the authentication middleware
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			claims := token.Claims.(jwt.MapClaims)
			for key, value := range claims {
				fmt.Printf("Claim[%s]: %v\n", key, value)
			}

			// aud := os.Getenv("AUTH0_API_IDENTIFIER")
			// checkAud := token.Claims.(jwt.MapClaims).VerifyAudience(aud, false)
			// if !checkAud {
			// 	return token, errors.New("Invalid audience.")
			// }

			iss := "https://" + os.Getenv("AUTH0_DOMAIN") + "/"
			checkIss := token.Claims.(jwt.MapClaims).VerifyIssuer(iss, false)
			if !checkIss {
				return token, errors.New("Invalid issuer.")
			}

			cert, err := getPemCert(token)
			if err != nil {
				return nil, err
			}
			return jwt.ParseRSAPublicKeyFromPEM([]byte(cert))
		},
		SigningMethod: jwt.SigningMethodRS256,
		Extractor: func(r *http.Request) (string, error) {
			cookie, err := r.Cookie("token")
			if err != nil {
				return "", err
			}
			return cookie.Value, nil
		},
	})

	// Apply the JWT middleware to the mux
	protectedHandler := jwtMiddleware.Handler(corsHandler)

	// Start server
	http.ListenAndServe("0.0.0.0:8080", h2c.NewHandler(protectedHandler, &http2.Server{}))
}

// Helper function to fetch the JWT's signing certificate
func getPemCert(token *jwt.Token) (string, error) {
	cert := ""
	resp, err := http.Get("https://" + os.Getenv("AUTH0_DOMAIN") + "/.well-known/jwks.json")

	if err != nil {
		return cert, err
	}
	defer resp.Body.Close()

	var jwks struct {
		Keys []struct {
			Kty string   `json:"kty"`
			Kid string   `json:"kid"`
			Use string   `json:"use"`
			N   string   `json:"n"`
			E   string   `json:"e"`
			X5c []string `json:"x5c"`
		} `json:"keys"`
	}

	err = json.NewDecoder(resp.Body).Decode(&jwks)

	if err != nil {
		return cert, err
	}

	for k := range jwks.Keys {
		if token.Header["kid"] == jwks.Keys[k].Kid {
			cert = "-----BEGIN CERTIFICATE-----\n" + jwks.Keys[k].X5c[0] + "\n-----END CERTIFICATE-----"
		}
	}

	if cert == "" {
		return cert, errors.New("Unable to find appropriate key.")
	}

	return cert, nil
}
