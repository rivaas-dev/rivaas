package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"rivaas.dev/router"
)

func main() {
	r := router.New()

	// Example 1: BindJSON - Parse JSON request body
	r.POST("/api/users", func(c *router.Context) {
		type CreateUserRequest struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Age   int    `json:"age"`
		}

		var req CreateUserRequest
		if err := c.BindBody(&req); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Validate
		if req.Name == "" || req.Email == "" {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "name and email are required",
			})
			return
		}

		c.JSON(http.StatusCreated, map[string]any{
			"message": "User created successfully",
			"user": map[string]any{
				"name":  req.Name,
				"email": req.Email,
				"age":   req.Age,
			},
		})
	})

	// Example 2: BindQuery - Parse query parameters
	r.GET("/api/search", func(c *router.Context) {
		type SearchParams struct {
			Query    string   `query:"q"`
			Page     int      `query:"page"`
			PageSize int      `query:"page_size"`
			Tags     []string `query:"tags"`
			Active   bool     `query:"active"`
		}

		var params SearchParams
		if err := c.BindQuery(&params); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Apply defaults
		if params.Page == 0 {
			params.Page = 1
		}
		if params.PageSize == 0 {
			params.PageSize = 10
		}

		c.JSON(http.StatusOK, map[string]any{
			"query":     params.Query,
			"page":      params.Page,
			"page_size": params.PageSize,
			"tags":      params.Tags,
			"active":    params.Active,
			"results":   []string{"result1", "result2", "result3"}, // Mock results
		})
	})

	// Example 3: BindParams - Parse URL path parameters
	r.GET("/api/users/:id/posts/:post_id", func(c *router.Context) {
		type PathParams struct {
			UserID int `params:"id"`
			PostID int `params:"post_id"`
		}

		var params PathParams
		if err := c.BindParams(&params); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid user ID or post ID",
			})
			return
		}

		c.JSON(http.StatusOK, map[string]any{
			"user_id": params.UserID,
			"post_id": params.PostID,
			"post": map[string]any{
				"id":      params.PostID,
				"user_id": params.UserID,
				"title":   "Sample Post",
			},
		})
	})

	// Example 4: BindCookies - Parse cookies
	r.GET("/api/session", func(c *router.Context) {
		type SessionCookies struct {
			SessionID  string `cookie:"session_id"`
			Theme      string `cookie:"theme"`
			RememberMe bool   `cookie:"remember_me"`
		}

		var cookies SessionCookies
		if err := c.BindCookies(&cookies); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		if cookies.SessionID == "" {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "No session found",
			})
			return
		}

		c.JSON(http.StatusOK, map[string]any{
			"session_id":  cookies.SessionID,
			"theme":       cookies.Theme,
			"remember_me": cookies.RememberMe,
			"status":      "authenticated",
		})
	})

	// Example 5: BindHeaders - Parse request headers
	r.GET("/api/client-info", func(c *router.Context) {
		type ClientHeaders struct {
			UserAgent     string `header:"User-Agent"`
			Authorization string `header:"Authorization"`
			Accept        string `header:"Accept"`
			AcceptLang    string `header:"Accept-Language"`
		}

		var headers ClientHeaders
		if err := c.BindHeaders(&headers); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, map[string]any{
			"user_agent": headers.UserAgent,
			"has_auth":   headers.Authorization != "",
			"accept":     headers.Accept,
			"language":   headers.AcceptLang,
		})
	})

	// Example 6: BindForm - Parse form data
	r.POST("/api/login", func(c *router.Context) {
		type LoginForm struct {
			Username string `form:"username"`
			Password string `form:"password"`
			Remember bool   `form:"remember"`
		}

		var form LoginForm
		if err := c.BindForm(&form); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		if form.Username == "" || form.Password == "" {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "username and password are required",
			})
			return
		}

		c.JSON(http.StatusOK, map[string]any{
			"message":  "Login successful",
			"username": form.Username,
			"remember": form.Remember,
			"token":    "mock-jwt-token",
		})
	})

	// Example 7: Combined binding - Use multiple sources
	r.POST("/api/users/:id/update", func(c *router.Context) {
		type UpdateRequest struct {
			// From URL params
			UserID int `params:"id"`

			// From query string
			Notify bool `query:"notify"`

			// From JSON body
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		var req UpdateRequest

		// Bind URL parameters
		if err := c.BindParams(&req); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid user ID",
			})
			return
		}

		// Bind query parameters
		if err := c.BindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Bind JSON body
		if err := c.BindBody(&req); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, map[string]any{
			"message": "User updated successfully",
			"user_id": req.UserID,
			"name":    req.Name,
			"email":   req.Email,
			"notify":  req.Notify,
		})
	})

	// Example 8: Optional fields with pointers
	r.GET("/api/products", func(c *router.Context) {
		type ProductFilters struct {
			Category *string  `query:"category"`
			MinPrice *float64 `query:"min_price"`
			MaxPrice *float64 `query:"max_price"`
			InStock  *bool    `query:"in_stock"`
			Page     int      `query:"page"`
			PageSize int      `query:"page_size"`
		}

		var filters ProductFilters
		if err := c.BindQuery(&filters); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Pointers allow distinguishing between "not provided" and "zero value"
		response := map[string]any{
			"page":      filters.Page,
			"page_size": filters.PageSize,
			"filters":   make(map[string]any),
		}

		if filters.Category != nil {
			response["filters"].(map[string]any)["category"] = *filters.Category
		}
		if filters.MinPrice != nil {
			response["filters"].(map[string]any)["min_price"] = *filters.MinPrice
		}
		if filters.MaxPrice != nil {
			response["filters"].(map[string]any)["max_price"] = *filters.MaxPrice
		}
		if filters.InStock != nil {
			response["filters"].(map[string]any)["in_stock"] = *filters.InStock
		}

		response["products"] = []string{"product1", "product2"}

		c.JSON(http.StatusOK, response)
	})

	// Example 9: Slices and arrays
	r.GET("/api/batch", func(c *router.Context) {
		type BatchRequest struct {
			IDs    []int     `query:"ids"`
			Tags   []string  `query:"tags"`
			Scores []float64 `query:"scores"`
		}

		var req BatchRequest
		if err := c.BindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, map[string]any{
			"message":   "Batch request processed",
			"id_count":  len(req.IDs),
			"tag_count": len(req.Tags),
			"ids":       req.IDs,
			"tags":      req.Tags,
			"scores":    req.Scores,
		})
	})

	// Example 10: Enhanced types - time.Time, time.Duration, net.IP, url.URL
	r.GET("/api/advanced-types", func(c *router.Context) {
		type AdvancedParams struct {
			// Time types
			StartDate time.Time     `query:"start"`
			EndDate   time.Time     `query:"end"`
			Timeout   time.Duration `query:"timeout"`

			// Network types
			AllowedIP net.IP  `query:"allowed_ip"`
			ProxyURL  url.URL `query:"proxy"`

			// Slices of advanced types
			Dates []time.Time `query:"dates"`
			IPs   []net.IP    `query:"ips"`

			// Optional advanced types
			OptionalDate *time.Time `query:"optional_date"`
		}

		var params AdvancedParams
		if err := c.BindQuery(&params); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		response := map[string]any{
			"start_date": params.StartDate.Format(time.RFC3339),
			"timeout":    params.Timeout.String(),
		}

		if !params.EndDate.IsZero() {
			response["end_date"] = params.EndDate.Format(time.RFC3339)
		}
		if params.AllowedIP != nil {
			response["allowed_ip"] = params.AllowedIP.String()
		}
		if params.ProxyURL.String() != "" {
			response["proxy_url"] = params.ProxyURL.String()
		}
		if len(params.Dates) > 0 {
			response["date_count"] = len(params.Dates)
		}
		if len(params.IPs) > 0 {
			response["ip_count"] = len(params.IPs)
		}

		c.JSON(http.StatusOK, response)
	})

	// Example 11: Embedded structs for code reuse
	r.GET("/api/embedded", func(c *router.Context) {
		type Pagination struct {
			Page     int `query:"page"`
			PageSize int `query:"page_size"`
		}

		type SearchRequest struct {
			Pagination        // Embedded struct
			Query      string `query:"q"`
			Sort       string `query:"sort"`
		}

		var req SearchRequest
		if err := c.BindQuery(&req); err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, map[string]any{
			"query":     req.Query,
			"sort":      req.Sort,
			"page":      req.Page,     // From embedded Pagination
			"page_size": req.PageSize, // From embedded Pagination
		})
	})

	// Example 12: Error handling with detailed feedback
	r.POST("/api/validate", func(c *router.Context) {
		type ValidationRequest struct {
			Email string  `json:"email"`
			Age   int     `json:"age"`
			Score float64 `json:"score"`
		}

		var req ValidationRequest
		if err := c.BindBody(&req); err != nil {
			// BindError provides detailed information
			c.JSON(http.StatusBadRequest, map[string]string{
				"error":   "Validation failed",
				"details": err.Error(),
			})
			return
		}

		// Business validation
		errors := make(map[string]string)
		if req.Email == "" {
			errors["email"] = "Email is required"
		}
		if req.Age < 18 {
			errors["age"] = "Must be 18 or older"
		}
		if req.Score < 0 || req.Score > 100 {
			errors["score"] = "Score must be between 0 and 100"
		}

		if len(errors) > 0 {
			c.JSON(http.StatusUnprocessableEntity, map[string]any{
				"error":  "Validation failed",
				"fields": errors,
			})
			return
		}

		c.JSON(http.StatusOK, map[string]string{
			"message": "Validation successful",
		})
	})

	log.Println("Server starting on :8080")
	log.Println("\n📚 Try these examples:")
	log.Println("\n1. BindJSON (POST with JSON):")
	log.Println(`  curl -X POST http://localhost:8080/api/users \`)
	log.Println(`    -H "Content-Type: application/json" \`)
	log.Println(`    -d '{"name":"Alice","email":"alice@example.com","age":25}'`)

	log.Println("\n2. BindQuery (GET with query params):")
	log.Println(`  curl "http://localhost:8080/api/search?q=golang&page=2&page_size=20&tags=web&tags=api&active=true"`)

	log.Println("\n3. BindParams (URL path params):")
	log.Println(`  curl http://localhost:8080/api/users/123/posts/456`)

	log.Println("\n4. BindCookies (with cookies):")
	log.Println(`  curl http://localhost:8080/api/session \`)
	log.Println(`    --cookie "session_id=abc123;theme=dark;remember_me=true"`)

	log.Println("\n5. BindHeaders (request headers):")
	log.Println(`  curl http://localhost:8080/api/client-info \`)
	log.Println(`    -H "User-Agent: CustomClient/1.0" \`)
	log.Println(`    -H "Authorization: Bearer token123"`)

	log.Println("\n6. BindForm (form data):")
	log.Println(`  curl -X POST http://localhost:8080/api/login \`)
	log.Println(`    -d "username=alice&password=secret123&remember=true"`)

	log.Println("\n7. Combined binding (params + query + body):")
	log.Println(`  curl -X POST "http://localhost:8080/api/users/123/update?notify=true" \`)
	log.Println(`    -H "Content-Type: application/json" \`)
	log.Println(`    -d '{"name":"Bob","email":"bob@example.com"}'`)

	log.Println("\n8. Optional fields with pointers:")
	log.Println(`  curl "http://localhost:8080/api/products?category=electronics&min_price=10.50&in_stock=true"`)

	log.Println("\n9. Slices and arrays:")
	log.Println(`  curl "http://localhost:8080/api/batch?ids=1&ids=2&ids=3&tags=go&tags=rust&scores=9.5&scores=8.7"`)

	log.Println("\n10. Enhanced types (time, duration, IP, URL):")
	log.Println(`  curl "http://localhost:8080/api/advanced-types?start=2024-01-15T10:00:00Z&timeout=30s&allowed_ip=192.168.1.1&proxy=http://proxy.com"`)

	log.Println("\n11. Embedded structs:")
	log.Println(`  curl "http://localhost:8080/api/embedded?q=search&sort=name&page=2&page_size=20"`)

	log.Println("\n12. Error handling:")
	log.Println(`  curl -X POST http://localhost:8080/api/validate \`)
	log.Println(`    -H "Content-Type: application/json" \`)
	log.Println(`    -d '{"email":"test@example.com","age":25,"score":95.5}'`)

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
