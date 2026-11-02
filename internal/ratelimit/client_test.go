package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/arthkinq/order-flow-engine/internal/ratelimit"
	"github.com/ozontech/testo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type RateLimitSuite struct {
	testo.Suite[*testo.T]
}

func (RateLimitSuite) TestAllow_FailOpenOnUnreachableServer(t *testo.T) {
	client, err := ratelimit.NewGateKeeperClient("passthrough:///127.0.0.1:59999")
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	allowed := client.Allow(ctx, "cust-123", "create_order")
	assert.True(t, allowed, "Expected Fail-Open policy to return true on unreachable GateKeeper server")
}

func (RateLimitSuite) TestAllow_FailOpenOnNilConnection(t *testo.T) {
	var client ratelimit.GateKeeperClient
	allowed := client.Allow(context.Background(), "cust-nil", "create_order")
	assert.True(t, allowed, "Expected Fail-Open policy to return true when gRPC connection is nil")
	assert.NoError(t, client.Close())
}

func (RateLimitSuite) TestClose_ActiveConnection(t *testo.T) {
	client, err := ratelimit.NewGateKeeperClient("passthrough:///127.0.0.1:59999")
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)
}

func TestRateLimitSuite(t *testing.T) {
	testo.RunSuite(t, new(RateLimitSuite))
}
