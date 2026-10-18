// SPDX-License-Identifier: Apache-2.0

package app

import (
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
)

// CloseError is the result of an op that ends the connection: the dispatcher closes with Code
// and Reason (machine data only; the code name when empty).
type CloseError struct {
	Code   domain.CloseCode
	Reason string
}

// Error returns the code name and the reason.
func (e *CloseError) Error() string {
	return "gateway: close " + strconv.Itoa(int(e.Code)) + " " + e.Code.Name() + " " + e.Reason
}

// closeWith returns the CloseError of code with the code name as reason.
func closeWith(code domain.CloseCode) *CloseError {
	return &CloseError{Code: code, Reason: code.Name()}
}

// OpError is the result of an op that failed without ending the connection: the dispatcher
// answers error {op, code, detail, ref} and keeps the connection (04 section 6.2). Code is a
// problem code of 06; Detail and Ref are optional.
type OpError struct {
	Code, Detail, Ref string
}

// Error returns the problem code.
func (e *OpError) Error() string { return "gateway: op failed: " + e.Code }
