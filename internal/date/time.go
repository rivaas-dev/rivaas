package date

import (
	"time"
)

const timeLayout = "2006-01-02T15:04:05Z"

func FromTimePtr(t *time.Time) *Date {
	if t == nil {
		return nil
	}

	return &Date{Time: *t}
}

// ParseTime parses a string in to a time.Time
func ParseTime(str string) (*time.Time, error) {
	if str == "" {
		return nil, nil
	}

	t, err := time.Parse(timeLayout, str)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func FormatTimeToPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}

	s := FormatTime(*t)
	return &s
}

func FormatTime(t time.Time) string {
	return t.Format(timeLayout)
}
