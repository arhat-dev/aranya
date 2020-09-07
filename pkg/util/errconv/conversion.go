package errconv

import (
	"errors"

	"arhat.dev/pkg/wellknownerrors"

	"arhat.dev/aranya-proto/gopb"
)

func ToConnectivityError(err error) *gopb.Error {
	if err == nil {
		return nil
	}

	var (
		msg  = err.Error()
		kind = gopb.ERR_COMMON
	)

	switch {
	case errors.Is(err, wellknownerrors.ErrAlreadyExists):
		kind = gopb.ERR_ALREADY_EXISTS
	case errors.Is(err, wellknownerrors.ErrNotFound):
		kind = gopb.ERR_NOT_FOUND
	case errors.Is(err, wellknownerrors.ErrNotSupported):
		kind = gopb.ERR_NOT_SUPPORTED
	}

	return gopb.NewError(kind, msg)
}
