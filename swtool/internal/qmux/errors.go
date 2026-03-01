package qmux

import (
	"errors"
	"fmt"
)

// QMIError represents a QMI protocol error returned in result TLV 0x02.
type QMIError struct {
	Code uint16
	Name string
}

func (e *QMIError) Error() string {
	if e.Name != "" {
		return fmt.Sprintf("QMI error %d (%s)", e.Code, e.Name)
	}
	return fmt.Sprintf("QMI error %d", e.Code)
}

// Sentinel errors for common QMI protocol error codes.
// Codes from libqmi-1.38.0/src/libqmi-glib/qmi-errors.h.
var (
	ErrMalformedMessage    = &QMIError{Code: 1, Name: "malformed-message"}
	ErrNoMemory            = &QMIError{Code: 2, Name: "no-memory"}
	ErrInternal            = &QMIError{Code: 3, Name: "internal"}
	ErrAborted             = &QMIError{Code: 4, Name: "aborted"}
	ErrClientIdsExhausted  = &QMIError{Code: 5, Name: "client-ids-exhausted"}
	ErrInvalidClientID     = &QMIError{Code: 7, Name: "invalid-client-id"}
	ErrNoNetworkFound      = &QMIError{Code: 13, Name: "no-network-found"}
	ErrNotProvisioned      = &QMIError{Code: 16, Name: "not-provisioned"}
	ErrDeviceInUse         = &QMIError{Code: 23, Name: "device-in-use"}
	ErrNoEffect            = &QMIError{Code: 26, Name: "no-effect"}
	ErrInvalidArgument     = &QMIError{Code: 48, Name: "invalid-argument"}
	ErrNoEntry             = &QMIError{Code: 50, Name: "no-entry"}
	ErrDeviceNotReady      = &QMIError{Code: 52, Name: "device-not-ready"}
	ErrInfoUnavailable     = &QMIError{Code: 74, Name: "information-unavailable"}
)

// qmiErrors maps known QMI error codes to sentinel errors.
var qmiErrors = map[uint16]*QMIError{
	1:  ErrMalformedMessage,
	2:  ErrNoMemory,
	3:  ErrInternal,
	4:  ErrAborted,
	5:  ErrClientIdsExhausted,
	7:  ErrInvalidClientID,
	13: ErrNoNetworkFound,
	16: ErrNotProvisioned,
	23: ErrDeviceInUse,
	26: ErrNoEffect,
	48: ErrInvalidArgument,
	50: ErrNoEntry,
	52: ErrDeviceNotReady,
	74: ErrInfoUnavailable,
}

// newQMIError returns a sentinel *QMIError for known codes, or a new
// *QMIError for unknown codes. Callers can use errors.Is to check
// against sentinels.
func newQMIError(code uint16) *QMIError {
	if e, ok := qmiErrors[code]; ok {
		return e
	}
	return &QMIError{Code: code}
}

// IsQMIError returns true if err is a *QMIError with the given code.
func IsQMIError(err error, code uint16) bool {
	var qe *QMIError
	if errors.As(err, &qe) {
		return qe.Code == code
	}
	return false
}
