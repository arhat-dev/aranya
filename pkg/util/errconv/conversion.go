package errconv

import (
	"errors"

	"arhat.dev/pkg/wellknownerrors"

	"arhat.dev/aranya-proto/aranyagopb"
)

func ToConnectivityError(err error) *aranyagopb.Error {
	if err == nil {
		return nil
	}

	var (
		msg  = err.Error()
		kind = aranyagopb.ERR_COMMON
	)

	switch {
	case errors.Is(err, wellknownerrors.ErrAlreadyExists):
		kind = aranyagopb.ERR_ALREADY_EXISTS
	case errors.Is(err, wellknownerrors.ErrNotFound):
		kind = aranyagopb.ERR_NOT_FOUND
	case errors.Is(err, wellknownerrors.ErrNotSupported):
		kind = aranyagopb.ERR_NOT_SUPPORTED
	}

	return aranyagopb.NewError(kind, msg)
}
