package apikey

import (
	"fmt"
	"github.com/google/uuid"
	"gitlab.ci.fdmg.org/ci-api/admin-api/internal/validation"
	"gitlab.ci.fdmg.org/ci-api/cigourn"
)

const NameMaxLength = 30

var (
	// validFilters valid filter keywords in query string and corresponding validators
	validFilters = map[string]func(string) error{
		actorIDFilter: func(in string) error {
			if in == "" {
				return nil
			}

			_, err := cigourn.Parse(in)
			return err
		},
		environmentFilter: func(in string) error {
			if in == "" {
				return nil
			}

			if !validation.ValidateEnvironment(in) {
				return fmt.Errorf("invalid environment `%s`", in)
			}

			return nil
		},
		activeFilter: func(in string) error {
			if in == "" {
				return nil
			}
			_, err := paramToBool(in)
			return err
		},
		expiresAtFilter: func(in string) error {
			if in == "" {
				return nil
			}
			_, err := paramToDate(in)
			return err
		},
	}
	validMatch = map[string]func(string) error{
		nameFilter: func(in string) error {
			if len(in) > NameMaxLength {
				return fmt.Errorf("maximum length is %d, %d given", NameMaxLength, len(in))
			}

			return nil
		}, // Description of the api key. Usually is the customer name and type of key, like Prod key customer X
		descriptionFilter: func(in string) error {
			return nil
		}, // Description of the api key. Usually is the customer name and type of key, like Prod key customer X
		customerIDFilter: func(in string) error {
			if in == "" {
				return nil
			}

			return uuid.Validate(in)
		}, // CustomerID is the ID of the overall Customer. Example: 19fd5d47-91ea-4cac-a8e0-ea9e295fa44b
		accountIDFilter: func(in string) error {
			if in == "" {
				return nil
			}

			return uuid.Validate(in)
		}, // AccountID is the ID of the account. An account can be for example API. Example: f82dddf1-4f06-4fa0-80bd-c2014d5f9540
	}
)

// ValidateFilters validates filter and match parameters
func ValidateFilters(param FilterParam) error {
	// ValidateFilters filters.
	for keyword, value := range param.Filter {
		validate, ok := validFilters[keyword]
		if !ok {
			return fmt.Errorf("invalid filter attribute `%s`", keyword)
		}

		err := validate(value)
		if err != nil {
			return fmt.Errorf("invalid value `%s` for `filter[%s]`: %s", value, keyword, err)
		}
	}
	// ValidateMatch filters.
	for keyword, value := range param.Match {
		validate, ok := validMatch[keyword]
		if !ok {
			return fmt.Errorf("invalid match attribute `%s`", keyword)
		}
		err := validate(value)
		if err != nil {
			return fmt.Errorf("invalid value `%s` for `match[%s]`: %s", value, keyword, err)
		}
	}
	return nil
}
