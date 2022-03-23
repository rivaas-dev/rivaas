package key

import (
	"errors"
)

const (
	DBKeyNotFoundText             = "key not found in database"
	DBKeyUpdateErrorText          = "could not update key"
	DBCommunicationErrorText      = "error while communicating with DB"
	CreateKeyGeneralErrorText     = "could not create key"
	GatewayKeyNotFoundText        = "key not found in gateway"
	GatewayCommunicationErrorText = "error while communicating gateway"
)

var (
	DBKeyNotFoundError      = DBKeyNotFound{errors.New(DBKeyNotFoundText)}
	GatewayKeyNotFoundError = GatewayKeyNotFound{errors.New(GatewayKeyNotFoundText)}
	GatewayCommError        = GatewayCommunicationError{errors.New(GatewayCommunicationErrorText)}
)

type DBKeyNotFound struct {
	error
}

type GatewayKeyNotFound struct {
	error
}

type GatewayCommunicationError struct {
	error
}
