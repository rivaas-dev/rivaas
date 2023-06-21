package validation

import "net/mail"

// ValidateEmail validates email addresses.
func ValidateEmail(emails []string) bool {
	for _, email := range emails {
		if _, err := mail.ParseAddress(email); err != nil {
			return false
		}
	}
	return true
}
