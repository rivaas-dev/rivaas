package handlers

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"rivaas.dev/router"
)

// CreateOrder creates a new order with full request binding and validation.
func CreateOrder(c *router.Context) {
	var req CreateOrderRequest
	if err := c.BindBody(&req); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "failed to parse request body: %v", err))
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		HandleError(c, err)
		return
	}

	// Add tracing
	c.SetSpanAttribute("operation", "create_order")
	c.SetSpanAttribute("order.user_id", req.UserID)
	c.SetSpanAttribute("order.currency", req.Currency)
	c.SetSpanAttribute("order.payment_method", req.PaymentMethod)
	c.AddSpanEvent("order_creation_started")

	// Simulate order processing with timing metrics
	start := time.Now()
	time.Sleep(50 * time.Millisecond)
	processingTime := time.Since(start).Seconds()

	// Calculate total
	var totalAmount float64
	for _, item := range req.Items {
		totalAmount += item.Price * float64(item.Quantity)
	}

	// Record custom metrics
	c.RecordMetric("order_processing_duration_seconds", processingTime,
		attribute.String("currency", req.Currency),
		attribute.String("payment_method", req.PaymentMethod),
	)

	c.IncrementCounter("orders_total",
		attribute.String("status", "success"),
		attribute.String("currency", req.Currency),
		attribute.String("payment_method", req.PaymentMethod),
	)

	c.AddSpanEvent("order_created")
	c.JSON(http.StatusCreated, OrderResponse{
		OrderID:        fmt.Sprintf("ord_%d", GenerateOrderID()),
		UserID:         req.UserID,
		Items:          req.Items,
		TotalAmount:    totalAmount,
		Currency:       req.Currency,
		Status:         "created",
		CreatedAt:      time.Now(),
		ProcessingTime: processingTime,
		Metadata:       req.Metadata,
	})
}

// GetOrderByID retrieves an order by ID.
func GetOrderByID(c *router.Context) {
	var params struct {
		ID int `params:"id"`
	}
	if err := c.BindParams(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("order.id", params.ID)
	c.SetSpanAttribute("operation", "get_order")

	// Simulate database lookup
	time.Sleep(10 * time.Millisecond)

	c.JSON(http.StatusOK, OrderResponse{
		OrderID:     fmt.Sprintf("ord_%d", params.ID),
		UserID:      123,
		Items:       []OrderItem{{ProductID: 1, Quantity: 2, Price: 29.99}},
		TotalAmount: 59.98,
		Currency:    "USD",
		Status:      "completed",
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	})
}
