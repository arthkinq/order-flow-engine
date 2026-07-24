// Package ratelimit provides an interface and gRPC client wrapper for GateKeeper rate limiter integration.
package ratelimit

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// LimiterClient defines the contract for checking client rate limits.
type LimiterClient interface {
	// Allow checks if the given client is allowed to access a resource.
	// Returns true if allowed, or if GateKeeper is unavailable (Fail-Open policy).
	Allow(ctx context.Context, clientID, resourceID string) bool
	Close() error
}

// GateKeeperClient implements LimiterClient over gRPC to the GateKeeper service.
type GateKeeperClient struct {
	conn   *grpc.ClientConn
	target string
}

// NewGateKeeperClient constructs a new gRPC client for GateKeeper.
func NewGateKeeperClient(target string) (*GateKeeperClient, error) {
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Printf("[GateKeeperClient] Warning: Failed to initialize gRPC client for %s: %v. Fail-open mode active.", target, err)
		return &GateKeeperClient{target: target}, nil
	}

	return &GateKeeperClient{
		conn:   conn,
		target: target,
	}, nil
}

// Allow checks rate limits with GateKeeper.
// Implements Fail-Open pattern.
func (c *GateKeeperClient) Allow(ctx context.Context, clientID, resourceID string) bool {
	if c.conn == nil {
		log.Printf("[GateKeeperClient] GateKeeper connection is nil for client [%s]. Failing open.", clientID)
		return true
	}

	callCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	var reply struct {
		Allowed   bool  `protobuf:"varint,1,opt,name=allowed"`
		Remaining int32 `protobuf:"varint,2,opt,name=remaining"`
		ResetTime int64 `protobuf:"varint,3,opt,name=reset_time"`
	}

	req := struct {
		ClientID   string `protobuf:"bytes,1,opt,name=client_id"`
		ResourceID string `protobuf:"bytes,2,opt,name=resource_id"`
	}{
		ClientID:   clientID,
		ResourceID: resourceID,
	}

	err := c.conn.Invoke(callCtx, "/ratelimit.v1.RateLimiter/ShouldAllow", &req, &reply)
	if err != nil {
		log.Printf("[GateKeeperClient] Rate check failed for client [%s]: %v. Failing open.", clientID, err)
		return true
	}

	return reply.Allowed
}

// Close closes the gRPC connection if active.
func (c *GateKeeperClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
