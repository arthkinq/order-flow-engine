// Package tests contains integration and end-to-end tests for Order Flow Engine.
package tests

import (
	"context"
	"testing"
	"time"

	orderv1 "github.com/arthkinq/order-flow-engine/proto/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestOrderFlow_Integration performs an end-to-end test against a running instance of Order Flow Engine.
func TestOrderFlow_Integration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Skipf("Skipping integration test: gRPC server not reachable at localhost:50051 (%v)", err)
	}
	defer conn.Close()

	client := orderv1.NewOrderServiceClient(conn)

	createReq := &orderv1.CreateOrderRequest{
		CustomerId: "integration-test-customer",
		Items: []*orderv1.OrderItem{
			{
				ItemId:     "item-keyboard",
				Quantity:   1,
				PriceCents: 8500,
			},
			{
				ItemId:     "item-mouse",
				Quantity:   2,
				PriceCents: 3500,
			},
		},
	}

	createResp, err := client.CreateOrder(ctx, createReq)
	if err != nil {
		t.Fatalf("CreateOrder failed: %v", err)
	}

	if createResp.GetOrderId() == "" {
		t.Fatal("Expected non-empty order_id in CreateOrderResponse")
	}

	t.Logf("Successfully created order with ID: %s", createResp.GetOrderId())

	getRespInitial, err := client.GetOrder(ctx, &orderv1.GetOrderRequest{
		OrderId: createResp.GetOrderId(),
	})
	if err != nil {
		t.Fatalf("GetOrder initial check failed: %v", err)
	}

	t.Logf("Initial order status: %s", getRespInitial.GetOrder().GetStatus())

	t.Log("Waiting for background worker processing...")
	time.Sleep(1 * time.Second)

	getRespFinal, err := client.GetOrder(ctx, &orderv1.GetOrderRequest{
		OrderId: createResp.GetOrderId(),
	})
	if err != nil {
		t.Fatalf("GetOrder final check failed: %v", err)
	}

	finalStatus := getRespFinal.GetOrder().GetStatus()
	t.Logf("Final order status after worker processing: %s", finalStatus)

	if finalStatus != orderv1.OrderStatus_ORDER_STATUS_COMPLETED && finalStatus != orderv1.OrderStatus_ORDER_STATUS_FAILED {
		t.Errorf("Expected final status to be COMPLETED or FAILED, got: %s", finalStatus)
	}
}
