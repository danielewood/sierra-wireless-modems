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

// Sentinel errors for common QMI error codes.
var (
	ErrMalformedMessage  = &QMIError{Code: 0x0001, Name: "malformed-message"}
	ErrNoMemory          = &QMIError{Code: 0x0002, Name: "no-memory"}
	ErrInternal          = &QMIError{Code: 0x0003, Name: "internal"}
	ErrInvalidArg        = &QMIError{Code: 0x0004, Name: "invalid-arg"}
	ErrNoEffect          = &QMIError{Code: 0x0005, Name: "no-effect"}
	ErrDeviceInUse       = &QMIError{Code: 0x0014, Name: "device-in-use"}
	ErrInvalidOperation  = &QMIError{Code: 0x0016, Name: "invalid-operation"}
	ErrAccessDenied      = &QMIError{Code: 0x0017, Name: "access-denied"}
	ErrNotProvisioned    = &QMIError{Code: 0x0019, Name: "not-provisioned"}
	ErrNotSupported      = &QMIError{Code: 0x001E, Name: "not-supported"}
	ErrNoFreeClient      = &QMIError{Code: 0x0024, Name: "no-free-client"}
	ErrInvalidClient     = &QMIError{Code: 0x0025, Name: "invalid-client"}
)

// qmiErrors maps known QMI error codes to sentinel errors.
var qmiErrors = map[uint16]*QMIError{
	0x0001: ErrMalformedMessage,
	0x0002: ErrNoMemory,
	0x0003: ErrInternal,
	0x0004: ErrInvalidArg,
	0x0005: ErrNoEffect,
	0x0014: ErrDeviceInUse,
	0x0016: ErrInvalidOperation,
	0x0017: ErrAccessDenied,
	0x0019: ErrNotProvisioned,
	0x001E: ErrNotSupported,
	0x0024: ErrNoFreeClient,
	0x0025: ErrInvalidClient,
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
