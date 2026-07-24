// Package server implements gRPC API handlers for OrderService.
package server

import (
	"context"
	"errors"

	"github.com/arthkinq/order-flow-engine/internal/domain"
	"github.com/arthkinq/order-flow-engine/internal/metrics"
	"github.com/arthkinq/order-flow-engine/internal/queue"
	"github.com/arthkinq/order-flow-engine/internal/ratelimit"
	"github.com/arthkinq/order-flow-engine/internal/repository"
	orderv1 "github.com/arthkinq/order-flow-engine/proto/order/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// OrderServer implements orderv1.OrderServiceServer.
type OrderServer struct {
	orderv1.UnimplementedOrderServiceServer
	repo      repository.OrderRepository
	publisher queue.Publisher
	limiter   ratelimit.LimiterClient
}

// NewOrderServer constructs a new gRPC OrderServer instance.
func NewOrderServer(
	repo repository.OrderRepository,
	publisher queue.Publisher,
	limiter ratelimit.LimiterClient,
) *OrderServer {
	return &OrderServer{
		repo:      repo,
		publisher: publisher,
		limiter:   limiter,
	}
}

// CreateOrder handles client request to place a new order.
func (s *OrderServer) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	customerID := req.GetCustomerId()

	if s.limiter != nil && !s.limiter.Allow(ctx, customerID, "create_order") {
		return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded for customer")
	}

	reqItems := req.GetItems()
	domainItems := make([]domain.OrderItem, 0, len(reqItems))
	for _, item := range reqItems {
		domainItems = append(domainItems, domain.OrderItem{
			ItemID:     item.GetItemId(),
			Quantity:   item.GetQuantity(),
			PriceCents: item.GetPriceCents(),
		})
	}

	order, err := domain.NewOrder(customerID, domainItems)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid order request: %v", err)
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save order: %v", err)
	}

	if err := s.publisher.PublishOrderCreated(ctx, order.ID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to publish order to queue: %v", err)
	}

	metrics.OrdersCreatedTotal.Inc()

	return &orderv1.CreateOrderResponse{
		OrderId:   order.ID,
		Status:    toProtoStatus(order.Status),
		CreatedAt: timestamppb.New(order.CreatedAt),
	}, nil
}

// GetOrder retrieves order details by order ID.
func (s *OrderServer) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	orderID := req.GetOrderId()
	if orderID == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "order [%s] not found", orderID)
		}
		return nil, status.Errorf(codes.Internal, "failed to fetch order: %v", err)
	}

	return &orderv1.GetOrderResponse{
		Order: toProtoOrder(order),
	}, nil
}

// Helper mapping function: domain.OrderStatus -> orderv1.OrderStatus
func toProtoStatus(s domain.OrderStatus) orderv1.OrderStatus {
	switch s {
	case domain.StatusPending:
		return orderv1.OrderStatus_ORDER_STATUS_PENDING
	case domain.StatusProcessing:
		return orderv1.OrderStatus_ORDER_STATUS_PROCESSING
	case domain.StatusCompleted:
		return orderv1.OrderStatus_ORDER_STATUS_COMPLETED
	case domain.StatusFailed:
		return orderv1.OrderStatus_ORDER_STATUS_FAILED
	default:
		return orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
}

// Helper mapping function: domain.Order -> orderv1.Order
func toProtoOrder(o *domain.Order) *orderv1.Order {
	items := make([]*orderv1.OrderItem, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, &orderv1.OrderItem{
			ItemId:     item.ItemID,
			Quantity:   item.Quantity,
			PriceCents: item.PriceCents,
		})
	}

	return &orderv1.Order{
		Id:               o.ID,
		CustomerId:       o.CustomerID,
		Items:            items,
		TotalAmountCents: o.TotalAmountCents,
		Status:           toProtoStatus(o.Status),
		FailureReason:    o.FailureReason,
		CreatedAt:        timestamppb.New(o.CreatedAt),
		UpdatedAt:        timestamppb.New(o.UpdatedAt),
	}
}
