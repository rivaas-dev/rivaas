package date

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestYmdDate_AllCases(t *testing.T) {
	a := assert.New(t)
	stringDate := "2008-04-12"
	ymdDate := YmdDate{}
	err := ymdDate.UnmarshalJSON([]byte("bla"))
	a.Error(err)
	err = ymdDate.UnmarshalJSON([]byte(stringDate))
	a.Nil(err)

	a.Equal(stringDate, ymdDate.String())
	b, err := ymdDate.MarshalJSON()
	a.Nil(err)
	a.Equal(fmt.Sprintf("\"%s\"", stringDate), string(b))

	l, err := CreateYmdFromString("2008-04-29")
	a.Nil(err)
	a.Equal("2008-04-29", l.String())
}
