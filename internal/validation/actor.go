package validation

import (
	"errors"
	"regexp"
)

func ValidateActorID(actorID, customerID string) error {
	// checks for an actor ID in both the header and body
	if actorID == "" && customerID == "" {
		return errors.New("no customer ID set in header or body")
	}

	re := regexp.MustCompile(`^[a-zA-Z]+:(\d+):(\d+)$`)
	if re.MatchString(customerID) {
		return nil
	} else {
		return errors.New("customer id does not match the right pattern")
	}
}
