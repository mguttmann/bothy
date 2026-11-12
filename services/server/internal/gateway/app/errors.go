// SPDX-License-Identifier: Apache-2.0

package app

import (
	"strconv"

	"github.com/OWNER/bothy/services/server/internal/gateway/domain"
	"github.com/OWNER/bothy/services/server/internal/kernel/errs"
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
// problem code of 06; Detail and Ref are optional. Detail is machine data in one of the forms of
// 04 6.2, never a value of the request and never prose: retryAfter with rate_limited, fieldDetail
// for the first field error, otherwise the reason of the problem or the name of the limit that
// was exceeded.
type OpError struct {
	Code, Detail, Ref string
}

// Error returns the problem code.
func (e *OpError) Error() string { return "gateway: op failed: " + e.Code }

// retryAfter is the machine text of a wait, "retry_after=<seconds>": the close reason of 4008 and
// the detail of an error frame with rate_limited (04 6.2 and 6.4).
func retryAfter(seconds int) string { return "retry_after=" + strconv.Itoa(seconds) }

// fieldDetail is the detail of an error frame for the first field error of e,
// "<pointer>: <code>" (04 6.2), and "" without one; never a value.
func fieldDetail(e *errs.Error) string {
	if len(e.Fields) == 0 {
		return ""
	}
	return e.Fields[0].Pointer + ": " + e.Fields[0].Code
}
