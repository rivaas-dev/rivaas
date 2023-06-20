package date

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Date represents custom date type.
type Date struct {
	time.Time
}

// dateFormat defines date format.
const dateFormat = "2006-01-02"

// UnmarshalJSON implements deserialization of the type.
func (d *Date) UnmarshalJSON(buf []byte) error {
	tt, err := time.Parse(dateFormat, strings.Trim(string(buf), `"`))
	if err != nil {
		return errors.New("invalid date format, should be `Y-m-d`")
	}
	d.Time = tt
	return nil
}

// MarshalJSON implements serialization of the type.
func (d *Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format(dateFormat))
}

// String implements string conversion of the type.
func (d *Date) String() string {
	return d.Time.Format(dateFormat)
}

// Scan implements the sql.Scanner interface.
func (d *Date) Scan(v any) error {
	d.Time = v.(time.Time)
	return nil
}

// GormDataType implements GormDataType method for the Date type.
func (d Date) GormDataType() string {
	return "Date"
}

// GormValue implements GormValue method for the Date type.
func (d Date) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	return clause.Expr{
		SQL:  "?",
		Vars: []interface{}{d.Time.Format(dateFormat)},
	}
}
