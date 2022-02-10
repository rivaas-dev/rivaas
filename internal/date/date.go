package date

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const YmdFormat = "2006-01-02"

//YmdDate JSON marshallable Y-m-d date
type YmdDate struct {
	time.Time
}

//UnmarshalJSON for automatic unpacking
func (d *YmdDate) UnmarshalJSON(buf []byte) error {
	tt, err := time.Parse(YmdFormat, strings.Trim(string(buf), `"`))
	if err != nil {
		return errors.New("invalid date format, should be `Y-m-d`")
	}
	d.Time = tt
	return nil
}

//MarshalJSON for automatic unpacking
func (d *YmdDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

//String toString
func (d *YmdDate) String() string {
	return d.Time.Format(YmdFormat)
}

func CreateYmdFromString(dateString string) (*YmdDate, error) {
	d := YmdDate{}
	err := d.UnmarshalJSON([]byte(dateString))
	return &d, err
}
