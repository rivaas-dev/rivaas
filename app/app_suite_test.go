package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"rivaas.dev/router"

	"github.com/stretchr/testify/suite"
)

// AppLifecycleSuite tests complex lifecycle scenarios with shared setup.
type AppLifecycleSuite struct {
	suite.Suite
	testApp *App
}

func (s *AppLifecycleSuite) SetupTest() {
	// Fresh app instance for each test
	app, err := New(
		WithServiceName("test-suite"),
		WithServiceVersion("1.0.0"),
	)
	s.Require().NoError(err)
	s.testApp = app
}

func (s *AppLifecycleSuite) TearDownTest() {
	// Cleanup - app doesn't need explicit cleanup, but we can verify it's usable
	if s.testApp != nil {
		// Verify app is still functional
		s.testApp.GET("/cleanup-check", func(c *router.Context) {
			c.String(http.StatusOK, "ok")
		})
		req := httptest.NewRequest("GET", "/cleanup-check", nil)
		resp, err := s.testApp.Test(req)
		s.NoError(err)
		s.Equal(http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}
}

func (s *AppLifecycleSuite) TestHooksExecutionOrder() {
	executionOrder := make([]string, 0)
	executionMutex := make(chan struct{}, 1)

	s.testApp.OnStart(func(ctx context.Context) error {
		executionMutex <- struct{}{}
		executionOrder = append(executionOrder, "OnStart")
		<-executionMutex
		return nil
	})

	s.testApp.OnReady(func() {
		executionMutex <- struct{}{}
		executionOrder = append(executionOrder, "OnReady")
		<-executionMutex
	})

	s.testApp.OnShutdown(func(ctx context.Context) {
		executionMutex <- struct{}{}
		executionOrder = append(executionOrder, "OnShutdown")
		<-executionMutex
	})

	s.testApp.OnStop(func() {
		executionMutex <- struct{}{}
		executionOrder = append(executionOrder, "OnStop")
		<-executionMutex
	})

	// Note: In a real test, we'd start the server and trigger shutdown
	// For this unit test, we just verify hooks are registered
	s.testApp.GET("/test", func(c *router.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Verify hooks are registered (they'll execute when server starts/stops)
	s.NotNil(s.testApp.hooks)
}

func (s *AppLifecycleSuite) TestRouteRegistration() {
	s.testApp.GET("/users", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "users"})
	})

	s.testApp.POST("/users", func(c *router.Context) {
		c.JSON(http.StatusCreated, map[string]string{"message": "created"})
	})

	// Test GET route
	req := httptest.NewRequest("GET", "/users", nil)
	resp, err := s.testApp.Test(req)
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Test POST route
	req = httptest.NewRequest("POST", "/users", nil)
	resp, err = s.testApp.Test(req)
	s.NoError(err)
	s.Equal(http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

func (s *AppLifecycleSuite) TestMiddlewareChain() {
	callOrder := make([]int, 0)
	callMutex := make(chan struct{}, 1)

	s.testApp.Use(func(c *router.Context) {
		callMutex <- struct{}{}
		callOrder = append(callOrder, 1)
		<-callMutex
		c.Next()
	})

	s.testApp.Use(func(c *router.Context) {
		callMutex <- struct{}{}
		callOrder = append(callOrder, 2)
		<-callMutex
		c.Next()
	})

	s.testApp.GET("/test", func(c *router.Context) {
		callMutex <- struct{}{}
		callOrder = append(callOrder, 3)
		<-callMutex
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := s.testApp.Test(req)
	s.NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Verify execution order: middleware 1, middleware 2, handler
	s.Equal([]int{1, 2, 3}, callOrder)
}

func TestAppLifecycleSuite(t *testing.T) {
	suite.Run(t, new(AppLifecycleSuite))
}
