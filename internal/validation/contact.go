package validation

import "net/mail"

// ValidateEmail validates email addresses.
func ValidateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func ValidateEmails(emails []string) bool {
	for _, email := range emails {
		if !ValidateEmail(email) {
			return false
		}
	}
	return true
}
