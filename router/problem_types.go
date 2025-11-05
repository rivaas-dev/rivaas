package router

// Standard problem type URIs.
// Update the base URL to match your API documentation.
const (
	ProblemTypeBlank                = "about:blank"
	ProblemTypeValidation           = "https://api.example.com/problems/validation"
	ProblemTypeNotFound             = "https://api.example.com/problems/not-found"
	ProblemTypeMethodNotAllowed     = "https://api.example.com/problems/method-not-allowed"
	ProblemTypeUnauthorized         = "https://api.example.com/problems/unauthorized"
	ProblemTypeForbidden            = "https://api.example.com/problems/forbidden"
	ProblemTypeConflict             = "https://api.example.com/problems/conflict"
	ProblemTypeRateLimit            = "https://api.example.com/problems/rate-limit"
	ProblemTypeInternal             = "https://api.example.com/problems/internal-error"
	ProblemTypeBadRequest           = "https://api.example.com/problems/bad-request"
	ProblemTypeUnprocessable        = "https://api.example.com/problems/unprocessable-entity"
	ProblemTypeMalformedJSON        = "https://api.example.com/problems/malformed-json"
	ProblemTypeUnsupportedMediaType = "https://api.example.com/problems/unsupported-media-type"
	ProblemTypeTooLarge             = "https://api.example.com/problems/payload-too-large"
	ProblemTypePreconditionFailed   = "https://api.example.com/problems/precondition-failed"
)
