package router

// Standard problem type slugs.
// Configure base URL via router.WithProblemBaseURL() or app.WithProblemBaseURL().
// Use with c.ProblemType(slug) to resolve to full URI.
const (
	PTBlank                = "about:blank"
	PTValidation           = "validation-error"
	PTNotFound             = "not-found"
	PTMethodNotAllowed     = "method-not-allowed"
	PTUnauthorized         = "unauthorized"
	PTForbidden            = "forbidden"
	PTConflict             = "conflict"
	PTRateLimit            = "rate-limit-exceeded"
	PTInternal             = "internal-error"
	PTBadRequest           = "bad-request"
	PTUnprocessable        = "unprocessable-entity"
	PTMalformedJSON        = "malformed-json"
	PTUnsupportedMediaType = "unsupported-media-type"
	PTTooLarge             = "payload-too-large"
	PTPreconditionFailed   = "precondition-failed"
	PTNotReady             = "not-ready"
	PTNotHealthy           = "not-healthy"
)
