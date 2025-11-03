package handlers

import (
	"net/http"
	"time"

	"rivaas.dev/router"
)

// ListProducts lists products with query parameter binding.
func ListProducts(c *router.Context) {
	var params SearchParams
	if err := c.BindQuery(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("resource", "products")
	c.SetSpanAttribute("query.page", params.Page)
	c.SetSpanAttribute("query.page_size", params.PageSize)
	c.AddSpanEvent("fetching_products")

	// Simulate work
	time.Sleep(20 * time.Millisecond)

	products := []map[string]any{
		{"id": "1", "name": "Product A", "price": 29.99},
		{"id": "2", "name": "Product B", "price": 49.99},
		{"id": "3", "name": "Product C", "price": 19.99},
	}

	c.JSON(http.StatusOK, map[string]any{
		"products":  products,
		"page":      params.Page,
		"page_size": params.PageSize,
		"total":     len(products),
		"trace_id":  c.TraceID(),
	})
}

// GetProductByID retrieves a product by ID with custom constraint.
func GetProductByID(c *router.Context) {
	var params ProductPathParams
	if err := c.BindParams(&params); err != nil {
		HandleError(c, WrapError(ErrInvalidInput, "%v", err))
		return
	}

	c.SetSpanAttribute("product.id", params.ID)
	c.SetSpanAttribute("operation", "get_product")

	// Simulate database lookup
	time.Sleep(10 * time.Millisecond)

	c.JSON(http.StatusOK, map[string]any{
		"id":    params.ID,
		"name":  "Product " + params.ID,
		"price": 29.99,
	})
}
