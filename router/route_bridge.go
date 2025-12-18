// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package router

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"unsafe"

	"rivaas.dev/router/compiler"
	"rivaas.dev/router/route"
)

// Compile-time check that Router implements route.Registrar.
var _ route.Registrar = (*Router)(nil)

// Re-export mount options for convenience.
var (
	// InheritMiddleware makes the subrouter inherit parent router's global middleware.
	InheritMiddleware = route.InheritMiddleware

	// NamePrefix adds a prefix to all route names in the subrouter.
	NamePrefix = route.NamePrefix
)

// WithMiddleware adds additional middleware to the subrouter.
func WithMiddleware(m ...HandlerFunc) route.MountOption {
	handlers := make([]route.Handler, 0, len(m))
	for _, h := range m {
		handlers = append(handlers, h)
	}

	return route.WithMiddleware(handlers...)
}

// WithNotFound sets a custom 404 handler for the subrouter.
func WithNotFound(h HandlerFunc) route.MountOption {
	return route.WithNotFound(h)
}

// IsFrozen returns true if routes cannot be modified.
func (r *Router) IsFrozen() bool {
	return r.frozen.Load()
}

// IsWarmedUp returns true if Warmup() has been called.
func (r *Router) IsWarmedUp() bool {
	r.pendingRoutesMu.Lock()
	defer r.pendingRoutesMu.Unlock()

	return r.warmedUp
}

// AddPendingRoute adds a route to the pending routes list for deferred registration.
func (r *Router) AddPendingRoute(rt *route.Route) {
	r.pendingRoutesMu.Lock()
	r.pendingRoutes = append(r.pendingRoutes, rt)
	r.pendingRoutesMu.Unlock()
}

// RegisterRouteNow registers a route immediately (used after warmup).
func (r *Router) RegisterRouteNow(rt *route.Route) {
	rt.RegisterRoute()
}

// GetGlobalMiddleware returns a copy of the router's global middleware.
func (r *Router) GetGlobalMiddleware() []route.Handler {
	r.middlewareMu.RLock()
	defer r.middlewareMu.RUnlock()
	handlers := make([]route.Handler, 0, len(r.middleware))
	for _, h := range r.middleware {
		handlers = append(handlers, h)
	}

	return handlers
}

// RecordRouteRegistration records a route registration for metrics/diagnostics.
func (r *Router) RecordRouteRegistration(method, path string) {
	r.recordRouteRegistration(method, path)
}

// Emit emits a diagnostic event.
func (r *Router) Emit(kind route.DiagnosticKind, msg string, data map[string]any) {
	r.emit(DiagnosticKind(kind), msg, data)
}

// UpdateRouteInfo updates route info for introspection when constraints are added.
func (r *Router) UpdateRouteInfo(method, path, version string, update func(info *route.Info)) {
	r.routeTree.routesMutex.Lock()
	defer r.routeTree.routesMutex.Unlock()
	for i := range r.routeTree.routes {
		info := &r.routeTree.routes[i]
		if info.Method == method && info.Path == path && info.Version == version {
			update(info)
			break
		}
	}
}

// RegisterNamedRoute registers a named route for reverse routing.
func (r *Router) RegisterNamedRoute(name string, rt *route.Route) error {
	r.routeTree.routesMutex.Lock()
	defer r.routeTree.routesMutex.Unlock()
	if existing, ok := r.namedRoutes[name]; ok {
		return fmt.Errorf("duplicate route name: %s (existing: %s %s, new: %s %s)",
			name, existing.Method(), existing.Path(), rt.Method(), rt.Path())
	}
	r.namedRoutes[name] = rt

	return nil
}

// GetRouteCompiler returns the route compiler (for compiled route matching).
func (r *Router) GetRouteCompiler() *compiler.RouteCompiler {
	return r.routeCompiler
}

// UseCompiledRoutes returns whether compiled route matching is enabled.
func (r *Router) UseCompiledRoutes() bool {
	return r.useCompiledRoutes
}

// AddRouteToTree adds a route to the routing tree.
func (r *Router) AddRouteToTree(method, path string, handlers []route.Handler, constraints []route.Constraint) {
	handlerFuncs := convertHandlers(handlers)
	r.addRouteToTree(method, path, handlerFuncs, constraints)
}

// AddVersionRoute adds a route to a version-specific tree.
func (r *Router) AddVersionRoute(version, method, path string, handlers []route.Handler, constraints []route.Constraint) {
	handlerFuncs := convertHandlers(handlers)
	r.addVersionRoute(version, method, path, handlerFuncs, constraints)
}

// convertHandlers converts []route.Handler to []HandlerFunc, handling type variations.
func convertHandlers(handlers []route.Handler) []HandlerFunc {
	handlerFuncs := make([]HandlerFunc, 0, len(handlers))
	for _, h := range handlers {
		switch fn := h.(type) {
		case HandlerFunc:
			handlerFuncs = append(handlerFuncs, fn)
		case func(*Context):
			handlerFuncs = append(handlerFuncs, HandlerFunc(fn))
		default:
			handlerFuncs = append(handlerFuncs, h.(HandlerFunc))
		}
	}

	return handlerFuncs
}

// StoreRouteInfo stores route info for introspection.
func (r *Router) StoreRouteInfo(info route.Info) {
	r.routeTree.routesMutex.Lock()
	r.routeTree.routes = append(r.routeTree.routes, info)
	r.routeTree.routesMutex.Unlock()
}

// CacheRouteHandlers caches handlers on a compiled route with proper type conversion.
func (r *Router) CacheRouteHandlers(compiledRoute *compiler.CompiledRoute, handlers []route.Handler) {
	handlerFuncs := convertHandlers(handlers)
	compiledRoute.SetCachedHandlers(unsafe.Pointer(&handlerFuncs))
}

// AddRouteWithConstraints adds a route with support for parameter constraints.
func (r *Router) AddRouteWithConstraints(method, path string, handlers []route.Handler) *route.Route {
	handlerFuncs := convertHandlers(handlers)
	return r.addRouteInternal(method, path, handlerFuncs)
}

// addRouteInternal is the internal implementation that creates a route.Route.
func (r *Router) addRouteInternal(method, path string, handlers []HandlerFunc) *route.Route {
	// Fail-fast: prevent route registration after server has started serving.
	// This design ensures routes are registered during a single-threaded configuration
	// phase, eliminating data races between route registration and request serving.
	if r.serving.Load() {
		panic(fmt.Sprintf("router: cannot register route %s %s after server has started serving requests.\n"+
			"Routes must be registered before calling ServeHTTP, Start, or Freeze.\n"+
			"This is a design constraint to prevent data races.", method, path))
	}
	if r.frozen.Load() {
		panic(fmt.Sprintf("router: cannot register route %s %s after router has been frozen.\n"+
			"Routes must be registered before calling Freeze.", method, path))
	}

	handlerName := "anonymous"
	if len(handlers) > 0 {
		handlerName = getHandlerName(handlers[len(handlers)-1])
	}

	var middlewareNames []string
	if len(handlers) > 1 {
		middlewareNames = make([]string, 0, len(handlers)-1)
		for i := range len(handlers) - 1 {
			middlewareNames = append(middlewareNames, getHandlerName(handlers[i]))
		}
	}

	paramCount := strings.Count(path, ":")
	if paramCount > 8 {
		r.emit(DiagHighParamCount, "route has more than 8 parameters, using map storage instead of array", map[string]any{
			"method":         method,
			"path":           path,
			"param_count":    paramCount,
			"recommendation": "consider restructuring route to use query parameters or request body for additional data",
		})
	}

	isStatic := !strings.Contains(path, ":") && !strings.HasSuffix(path, "*")

	r.routeTree.routesMutex.Lock()
	r.routeTree.routes = append(r.routeTree.routes, route.Info{
		Method:      method,
		Path:        path,
		HandlerName: handlerName,
		Middleware:  middlewareNames,
		Constraints: make(map[string]string),
		IsStatic:    isStatic,
		Version:     "",
		ParamCount:  paramCount,
	})
	r.routeTree.routesMutex.Unlock()

	routeHandlers := make([]route.Handler, 0, len(handlers))
	for _, h := range handlers {
		routeHandlers = append(routeHandlers, h)
	}

	rt := route.NewRoute(r, "", method, path, routeHandlers)
	r.recordRouteRegistration(method, path)

	r.pendingRoutesMu.Lock()
	if r.warmedUp {
		r.pendingRoutesMu.Unlock()
		rt.RegisterRoute()
	} else {
		r.pendingRoutes = append(r.pendingRoutes, rt)
		r.pendingRoutesMu.Unlock()
	}

	return rt
}

func getHandlerName(handler HandlerFunc) string {
	if handler == nil {
		return "nil"
	}

	funcPtr := runtime.FuncForPC(reflect.ValueOf(handler).Pointer())
	if funcPtr == nil {
		return "unknown"
	}

	fullName := funcPtr.Name()
	file, line := funcPtr.FileLine(funcPtr.Entry())

	// Clean the function name (strip arguments, simplify anonymous)
	cleanName := cleanHandlerFuncName(fullName)

	// Shorten file path for readability
	shortFile := shortenFilePath(file)

	return fmt.Sprintf("%s (%s:%d)", cleanName, shortFile, line)
}

// cleanHandlerFuncName strips argument info and simplifies anonymous functions.
func cleanHandlerFuncName(fn string) string {
	// Strip "created by " prefix if present
	fn = strings.TrimPrefix(fn, "created by ")

	// Strip argument patterns (e.g., "(0x...", "(...)", "({...}")
	argPatterns := []string{"(0x", "(...)", "({", "()"}
	for _, pattern := range argPatterns {
		if idx := strings.LastIndex(fn, pattern); idx > 0 {
			fn = fn[:idx]
			break
		}
	}

	// Simplify anonymous functions (FuncName.func1.2 -> FuncName(λ))
	fn = simplifyAnonFuncName(fn)

	// Ensure function names end with () for consistency (except lambda)
	if len(fn) > 0 && !strings.HasSuffix(fn, ")") {
		fn += "()"
	}

	return fn
}

// simplifyAnonFuncName converts Go's anonymous function names to cleaner format.
// e.g., "package.Func.func1.2" -> "package.Func(λ)"
func simplifyAnonFuncName(fn string) string {
	idx := strings.Index(fn, ".func")
	if idx <= 0 {
		return fn
	}

	// Verify suffix is only digits and dots (valid anonymous function pattern)
	suffix := fn[idx+5:]
	if len(suffix) == 0 {
		return fn
	}
	for _, c := range suffix {
		if c != '.' && (c < '0' || c > '9') {
			return fn
		}
	}

	return fn[:idx] + "(λ)"
}

// shortenFilePath shortens a file path for readability.
// Prioritizes showing the most relevant parts: package/file.go
func shortenFilePath(path string) string {
	// Just return the base filename for simplicity
	return filepath.Base(path)
}

// Group creates a new route group with the specified prefix and optional middleware.
func (r *Router) Group(prefix string, middleware ...HandlerFunc) *route.Group {
	handlers := make([]route.Handler, 0, len(middleware))
	for _, h := range middleware {
		handlers = append(handlers, h)
	}

	return route.NewGroup(r, prefix, handlers)
}

// Mount mounts a subrouter at the given prefix.
func (r *Router) Mount(prefix string, sub *Router, opts ...route.MountOption) {
	if sub == nil {
		return
	}

	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" || prefix[0] != '/' {
		prefix = "/" + prefix
	}

	cfg := route.BuildMountConfig(opts...)

	var middlewareChain []HandlerFunc
	if cfg.InheritMiddleware {
		r.middlewareMu.RLock()
		middlewareChain = make([]HandlerFunc, 0, len(r.middleware))
		middlewareChain = append(middlewareChain, r.middleware...)
		r.middlewareMu.RUnlock()
	}

	sub.middlewareMu.RLock()
	middlewareChain = append(middlewareChain, sub.middleware...)
	sub.middlewareMu.RUnlock()

	for _, h := range cfg.ExtraMiddleware {
		middlewareChain = append(middlewareChain, h.(HandlerFunc))
	}

	r.mergeSubrouterRoutes(prefix, sub, middlewareChain, cfg.NamePrefix)

	if cfg.NotFoundHandler != nil {
		notFoundHandler := cfg.NotFoundHandler.(HandlerFunc)
		originalNoRoute := r.noRouteHandler
		r.NoRoute(func(c *Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, prefix) {
				notFoundHandler(c)
			} else if originalNoRoute != nil {
				originalNoRoute(c)
			} else {
				c.Status(http.StatusNotFound)
			}
		})
	}
}

func (r *Router) mergeSubrouterRoutes(prefix string, sub *Router, middlewareChain []HandlerFunc, namePrefix string) {
	sub.pendingRoutesMu.Lock()
	pendingRoutes := make([]*route.Route, 0, len(sub.pendingRoutes))
	pendingRoutes = append(pendingRoutes, sub.pendingRoutes...)
	sub.pendingRoutesMu.Unlock()

	for _, rt := range pendingRoutes {
		r.mountRoute(prefix, rt, middlewareChain, namePrefix)
	}

	sub.routeTree.routesMutex.RLock()
	routeInfos := make([]route.Info, 0, len(sub.routeTree.routes))
	routeInfos = append(routeInfos, sub.routeTree.routes...)
	sub.routeTree.routesMutex.RUnlock()

	if len(routeInfos) > 0 && len(pendingRoutes) == 0 {
		r.mergeFromSubrouterTrees(prefix, sub, middlewareChain, namePrefix)
	}
}

func (r *Router) mountRoute(prefix string, rt *route.Route, middlewareChain []HandlerFunc, namePrefix string) {
	var fullPath string
	if rt.Path() == "/" {
		fullPath = prefix
	} else {
		fullPath = prefix + rt.Path()
	}

	routeHandlers := rt.Handlers()
	allHandlers := make([]HandlerFunc, 0, len(middlewareChain)+len(routeHandlers))
	allHandlers = append(allHandlers, middlewareChain...)
	for _, h := range routeHandlers {
		allHandlers = append(allHandlers, h.(HandlerFunc))
	}

	newRoute := r.addRouteInternal(rt.Method(), fullPath, allHandlers)

	for _, constraint := range rt.Constraints() {
		pattern := route.ExtractConstraintPattern(constraint)
		newRoute.Where(constraint.Param, pattern)
	}

	for param, constraint := range rt.TypedConstraints() {
		switch constraint.Kind {
		case route.ConstraintInt:
			newRoute.WhereInt(param)
		case route.ConstraintFloat:
			newRoute.WhereFloat(param)
		case route.ConstraintUUID:
			newRoute.WhereUUID(param)
		case route.ConstraintRegex:
			newRoute.WhereRegex(param, constraint.Pattern)
		case route.ConstraintEnum:
			newRoute.WhereEnum(param, constraint.Enum...)
		case route.ConstraintDate:
			newRoute.WhereDate(param)
		case route.ConstraintDateTime:
			newRoute.WhereDateTime(param)
		}
	}

	if rt.Name() != "" {
		newRoute.SetName(namePrefix + rt.Name())
	}
	if rt.Description() != "" {
		newRoute.SetDescription(rt.Description())
	}
	if len(rt.Tags()) > 0 {
		newRoute.SetTags(rt.Tags()...)
	}
}

func (r *Router) mergeFromSubrouterTrees(prefix string, sub *Router, middlewareChain []HandlerFunc, namePrefix string) {
	treesPtr := sub.routeTree.loadTrees()
	if treesPtr == nil {
		return
	}
	trees := *treesPtr

	for method, tree := range trees {
		r.extractAndMountFromNode(prefix, method, "", tree, middlewareChain, namePrefix)
	}
}

func (r *Router) extractAndMountFromNode(prefix, method, currentPath string, n *node, middlewareChain []HandlerFunc, namePrefix string) {
	if n == nil {
		return
	}

	// No locking needed - this is called during the configuration phase (before serving)
	// when route registration is single-threaded.
	handlers := n.handlers
	nodePath := n.path
	constraints := n.constraints
	children := n.children
	paramNode := n.param
	wildcardNode := n.wildcard

	if len(handlers) > 0 && nodePath != "" {
		fullPath := prefix + nodePath

		allHandlers := make([]HandlerFunc, 0, len(middlewareChain)+len(handlers))
		allHandlers = append(allHandlers, middlewareChain...)
		allHandlers = append(allHandlers, handlers...)

		newRoute := r.addRouteInternal(method, fullPath, allHandlers)

		if len(constraints) > 0 {
			for _, constraint := range constraints {
				pattern := constraint.Pattern.String()
				if len(pattern) >= 2 && pattern[0] == '^' && pattern[len(pattern)-1] == '$' {
					pattern = pattern[1 : len(pattern)-1]
				}
				newRoute.Where(constraint.Param, pattern)
			}
		}
	}

	for _, child := range children {
		r.extractAndMountFromNode(prefix, method, currentPath, child, middlewareChain, namePrefix)
	}

	if paramNode != nil && paramNode.node != nil {
		r.extractAndMountFromNode(prefix, method, currentPath, paramNode.node, middlewareChain, namePrefix)
	}

	if wildcardNode != nil && wildcardNode.node != nil {
		r.extractAndMountFromNode(prefix, method, currentPath, wildcardNode.node, middlewareChain, namePrefix)
	}
}

// URLFor generates a URL from a route name and parameters.
func (r *Router) URLFor(routeName string, params map[string]string, query url.Values) (string, error) {
	if !r.frozen.Load() {
		return "", ErrRoutesNotFrozen
	}

	r.routeTree.routesMutex.RLock()
	rt, ok := r.namedRoutes[routeName]
	r.routeTree.routesMutex.RUnlock()

	if !ok {
		return "", fmt.Errorf("%w: %s", ErrRouteNotFound, routeName)
	}

	if rt.ReversePattern() == nil {
		rt.SetReversePattern(route.ParseReversePattern(rt.Path()))
	}

	return rt.ReversePattern().BuildURL(params, query)
}

// MustURLFor generates a URL from a route name and parameters, panicking on error.
func (r *Router) MustURLFor(routeName string, params map[string]string, query url.Values) string {
	url, err := r.URLFor(routeName, params, query)
	if err != nil {
		panic(fmt.Sprintf("MustURLFor failed: %v", err))
	}

	return url
}

// Routes returns a list of all registered routes for introspection.
// The returned slice is sorted by method and then by path for consistency.
func (r *Router) Routes() []route.Info {
	r.routeTree.routesMutex.RLock()
	routes := make([]route.Info, 0, len(r.routeTree.routes))
	routes = append(routes, r.routeTree.routes...)
	r.routeTree.routesMutex.RUnlock()

	// Sort by method, then by path for consistent output
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Method == routes[j].Method {
			return routes[i].Path < routes[j].Path
		}

		return routes[i].Method < routes[j].Method
	})

	return routes
}

// Frozen returns true if the router has been frozen (routes are immutable).
func (r *Router) Frozen() bool {
	return r.frozen.Load()
}

// Freeze freezes the router, making all routes immutable.
// After Freeze is called:
//   - All pending routes are registered
//   - Routes are compiled for optimal lookups
//   - Any attempt to register new routes will panic
//
// Freeze is automatically called on the first ServeHTTP call.
// Call it explicitly for testing or when you need to access route information
// before serving requests.
//
// Freeze is safe to call from multiple goroutines concurrently.
// All callers will block until the freeze is complete.
func (r *Router) Freeze() {
	r.freezeOnce.Do(func() {
		// Mark as serving to prevent new route registrations
		r.serving.Store(true)
		r.frozen.Store(true)

		// First, register all pending routes (this is what Warmup does)
		r.Warmup()

		// Freeze the route compiler to bypass mutex in hot path
		// This must happen after Warmup() which adds routes to the compiler
		if r.routeCompiler != nil {
			r.routeCompiler.Freeze()
		}

		r.routeTree.routesMutex.Lock()
		for _, rt := range r.namedRoutes {
			if rt.ReversePattern() == nil {
				rt.SetReversePattern(route.ParseReversePattern(rt.Path()))
			}
		}

		routes := make([]*route.Route, 0, len(r.namedRoutes))
		for _, rt := range r.namedRoutes {
			routes = append(routes, rt)
		}

		r.routeSnapshotMutex.Lock()
		r.routeSnapshot = routes
		r.routeSnapshotMutex.Unlock()

		r.routeTree.routesMutex.Unlock()

		// Note: CompileAllRoutes is also called by Warmup, but it's safe to call again
		// (idempotent operation - routes are already compiled)
	})
}

// GetRoute retrieves a route by name.
func (r *Router) GetRoute(name string) (*route.Route, bool) {
	if !r.frozen.Load() {
		panic("routes not frozen yet; call Freeze() before accessing routes")
	}

	r.routeTree.routesMutex.RLock()
	rt, ok := r.namedRoutes[name]
	r.routeTree.routesMutex.RUnlock()

	return rt, ok
}

// GetRoutes returns an immutable snapshot of all named routes.
func (r *Router) GetRoutes() []*route.Route {
	if !r.frozen.Load() {
		panic("routes not frozen yet; call Freeze() before accessing routes")
	}

	r.routeSnapshotMutex.RLock()
	defer r.routeSnapshotMutex.RUnlock()

	result := make([]*route.Route, 0, len(r.routeSnapshot))
	result = append(result, r.routeSnapshot...)

	return result
}
