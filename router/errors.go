package router

import "errors"

// Static errors for better error handling and testing.
// These errors should be wrapped with fmt.Errorf and %w when context is needed.
var (
	// Binding errors are now in rivaas.dev/router/binding package

	// Context errors
	ErrContextReleased       = errors.New("context has been released - cannot write response")
	ErrContextResponseNil    = errors.New("context has been released - Response is nil")
	ErrProblemDetailNil      = errors.New("ProblemDetail called with nil problem")
	ErrContentTypeNotAllowed = errors.New("content type not allowed")

	// Request errors
	ErrFileNotFound = errors.New("file not found")
	ErrNoFilesFound = errors.New("no files found for key")

	// Router errors
	ErrResponseWriterNotHijacker = errors.New("responseWriter does not implement http.Hijacker")

	// Validation errors
	ErrCannotValidateNilValue     = errors.New("cannot validate nil value")
	ErrCannotValidateInvalidValue = errors.New("cannot validate invalid value")
	ErrUnknownValidationStrategy  = errors.New("unknown validation strategy")
	ErrCannotRegisterValidators   = errors.New("cannot register validators after first use")
	ErrValidationErrorNil         = errors.New("ValidationErrorRFC7807 called with nil error")

	// Test errors (for test files)
	ErrInvalidUUIDFormat    = errors.New("invalid UUID format: must be 36 characters")
	ErrReadError            = errors.New("read error")
	ErrBindingFailed        = errors.New("binding failed")
	ErrCookieNotFound       = errors.New("cookie not found")
	ErrUserIDRequired       = errors.New("user ID is required")
	ErrPageParameterInvalid = errors.New("page parameter is invalid")
	ErrInvalidType          = errors.New("invalid type")
	ErrCustomNameRequired   = errors.New("custom: name is required")
	ErrGenericValidation    = errors.New("generic validation error")
	ErrOuterError           = errors.New("outer error")
	ErrGenericError         = errors.New("generic error")
	ErrQueryInvalidInteger  = errors.New("query: invalid integer")
)
