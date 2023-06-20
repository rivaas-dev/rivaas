package validation

import (
	"time"

	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/date"
)

// ValidateDate validates end date to be greater now.
func ValidateEndDate(requestedDate *date.Date) bool {
	return requestedDate.After(time.Now())
}
