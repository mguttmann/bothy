// SPDX-License-Identifier: Apache-2.0

package domain

import (
	"bytes"
	"errors"
	"strings"
	"time"
)

// VerdictKind is the class of a clamd answer (docs/spec/12-files.md 5.2).
type VerdictKind string

// The verdict classes. A timeout or a connection error is no verdict: the scanner returns an
// error and the file stays pending (5.2 last row).
const (
	// VerdictClean is "stream: OK".
	VerdictClean VerdictKind = "clean"
	// VerdictInfected is "stream: <signature> FOUND" with a signature that is not a limit hit.
	VerdictInfected VerdictKind = "infected"
	// VerdictError is a limit hit (Heuristics.Limits.Exceeded...), the stream limit or any other
	// ERROR line.
	VerdictError VerdictKind = "error"
)

// Verdict is the parsed answer of clamd to one INSTREAM.
type Verdict struct {
	// Kind is the class of the answer.
	Kind VerdictKind
	// Signature is the signature name of a FOUND answer, "" otherwise. It is written only to the
	// audit log, normalised by AuditSignature; never to logs, spans or metrics (12 14).
	Signature string
	// Reason is the machine code of a blocked file (ReasonMalwareDetected, ReasonScanLimitsExceeded,
	// ReasonScanStreamLimitExceeded, ReasonScanEngineError); "" for clean.
	Reason string
}

// The reason codes of file_upload_sessions.failure_reason that the scan pass writes
// (docs/DECISIONS.md D-M3-04-3): machine codes without content (03 4.7).
const (
	ReasonSizeMismatch            = "size_mismatch"
	ReasonChecksumMismatch        = "checksum_mismatch"
	ReasonFileTypeDenied          = "file_type_denied"
	ReasonScanLimitsExceeded      = "scan_limits_exceeded"
	ReasonMalwareDetected         = "malware_detected"
	ReasonScanStreamLimitExceeded = "scan_stream_limit_exceeded"
	ReasonScanEngineError         = "scan_engine_error"
)

// ErrClamdProtocol reports a clamd answer outside the forms of 12 5.1 item 4. The scan pass treats
// it as transient: the file stays pending and the job retries (docs/DECISIONS.md D-M3-04-4).
var ErrClamdProtocol = errors.New("files: unexpected clamd reply")

// The fixed parts of a clamd INSTREAM answer (12 5.1, source Q7).
const (
	replyStream      = "stream: "
	replyOK          = "stream: OK"
	replyFound       = " FOUND"
	replyError       = " ERROR"
	replyLimitPrefix = "Heuristics.Limits.Exceeded"
	replySizeLimit   = "INSTREAM size limit exceeded"
)

// ParseClamdReply maps the answer of clamd to one zINSTREAM (12 5.2): "stream: OK" is clean;
// "stream: <signature> FOUND" is an error with ReasonScanLimitsExceeded when the signature starts
// with Heuristics.Limits.Exceeded, otherwise infected; "INSTREAM size limit exceeded. ERROR" is an
// error with ReasonScanStreamLimitExceeded and any other line ending in " ERROR" an error with
// ReasonScanEngineError. One trailing NUL is accepted (the z framing). Every other form, an empty
// reply, a reply with control bytes and a FOUND without a signature give ErrClamdProtocol. The
// text of an ERROR line is never kept.
//
// Example:
//
//	v, err := domain.ParseClamdReply([]byte("stream: OK\x00")) // VerdictClean
func ParseClamdReply(reply []byte) (Verdict, error) {
	reply = bytes.TrimSuffix(reply, []byte{0})
	for _, c := range reply {
		if c < 0x20 || c == 0x7f {
			return Verdict{}, ErrClamdProtocol
		}
	}
	s := string(reply)
	switch {
	case s == replyOK:
		return Verdict{Kind: VerdictClean}, nil
	case strings.HasSuffix(s, replyError):
		if strings.Contains(s, replySizeLimit) {
			return Verdict{Kind: VerdictError, Reason: ReasonScanStreamLimitExceeded}, nil
		}
		return Verdict{Kind: VerdictError, Reason: ReasonScanEngineError}, nil
	case len(s) > len(replyStream)+len(replyFound) && strings.HasPrefix(s, replyStream) && strings.HasSuffix(s, replyFound):
		sig := s[len(replyStream) : len(s)-len(replyFound)]
		if sig == "" || strings.TrimSpace(sig) != sig || strings.ContainsRune(sig, ' ') {
			return Verdict{}, ErrClamdProtocol
		}
		if strings.HasPrefix(sig, replyLimitPrefix) {
			return Verdict{Kind: VerdictError, Signature: sig, Reason: ReasonScanLimitsExceeded}, nil
		}
		return Verdict{Kind: VerdictInfected, Signature: sig, Reason: ReasonMalwareDetected}, nil
	}
	return Verdict{}, ErrClamdProtocol
}

// The values of files.upload_state and files.scan_status (03 4.7).
const (
	UploadPending  = "pending"
	UploadComplete = "complete"
	UploadFailed   = "failed"

	ScanPending  = "pending"
	ScanClean    = "clean"
	ScanInfected = "infected"
	ScanError    = "error"
	ScanSkipped  = "skipped"
)

// Deliverable reports whether a file may be served (12 2): upload_state complete, scan_status
// clean or skipped, and not deleted. M3-05 answers every other state with 409.
//
// Example:
//
//	ok := domain.Deliverable("complete", "clean", false) // true
func Deliverable(uploadState, scanStatus string, deleted bool) bool {
	return !deleted && uploadState == UploadComplete && (scanStatus == ScanClean || scanStatus == ScanSkipped)
}

// Final reports whether a file reached its end state (12 2, "final"): deliverable, or
// upload_state failed, or scan_status infected or error. Deletion is not a state of 12 2: a deleted
// file that was deliverable is not final, while failed, infected and error stay final.
//
// Example:
//
//	done := domain.Final("complete", "infected", false) // true
func Final(uploadState, scanStatus string, deleted bool) bool {
	return Deliverable(uploadState, scanStatus, deleted) || uploadState == UploadFailed ||
		scanStatus == ScanInfected || scanStatus == ScanError
}

// PassFacts are what the one read pass of 12 3.4 learned about a file.
type PassFacts struct {
	// DeclaredSize and DeclaredSHA256 come from the session.
	DeclaredSize   int64
	DeclaredSHA256 [32]byte
	// CountedSize and ComputedSHA256 come from the stream. Counting stops at the declared size
	// plus one byte (design note 4b), so a larger object reads as one byte too many.
	CountedSize    int64
	ComputedSHA256 [32]byte
	// TypeDenied reports that the detected type is listed in file.denied_mime_types.
	TypeDenied bool
	// ScanOff is file.scan.mode = off.
	ScanOff bool
	// Scanned reports that the scanner returned a verdict; false after a transient scanner error
	// or a reply ParseClamdReply refused.
	Scanned bool
	// Verdict is the verdict when Scanned.
	Verdict Verdict
}

// PassOutcome is the decision for one pass (brief M3-04 design note 5, docs/DECISIONS.md
// D-M3-04-3).
type PassOutcome struct {
	// Retry leaves the file pending and schedules the next round; every other field is zero.
	Retry bool
	// UploadState and ScanStatus are the new values of files.
	UploadState, ScanStatus string
	// FailureReason is file_upload_sessions.failure_reason, "" for none.
	FailureReason string
	// SetMIMEType stores the detected type as files.mime_type.
	SetMIMEType bool
	// DeleteObjects removes the object and the parts after the commit.
	DeleteObjects bool
	// AuditMalware writes the malware audit entry.
	AuditMalware bool
	// ScanEngine sets scanned_at and scan_engine (clamd returned a verdict that decided the file).
	ScanEngine bool
	// MetricOutcome is bothy.outcome of bothy.file.scan; "" counts nothing.
	MetricOutcome string
}

// DecidePass applies the outcome table of brief M3-04 design note 5; the first matching row wins.
// Size, checksum and denied type decide even when the scan failed transiently, because they are
// known from the complete stream; the malware entry is written for such a failed file as well
// when the scan found malware, so a wrong checksum cannot hide a detection.
//
// Example:
//
//	out := domain.DecidePass(domain.PassFacts{DeclaredSize: 3, CountedSize: 3, Scanned: true, Verdict: v})
func DecidePass(in PassFacts) PassOutcome {
	malware := in.Scanned && in.Verdict.Kind == VerdictInfected
	failed := func(reason string, mime bool) PassOutcome {
		return PassOutcome{UploadState: UploadFailed, ScanStatus: ScanPending, FailureReason: reason, SetMIMEType: mime,
			DeleteObjects: true, AuditMalware: malware}
	}
	switch {
	case in.CountedSize != in.DeclaredSize:
		return failed(ReasonSizeMismatch, false)
	case in.ComputedSHA256 != in.DeclaredSHA256:
		return failed(ReasonChecksumMismatch, false)
	case in.TypeDenied:
		return failed(ReasonFileTypeDenied, true)
	case in.ScanOff:
		return PassOutcome{UploadState: UploadComplete, ScanStatus: ScanSkipped, SetMIMEType: true, MetricOutcome: ScanSkipped}
	case !in.Scanned:
		return PassOutcome{Retry: true}
	}
	switch in.Verdict.Kind {
	case VerdictClean:
		return PassOutcome{UploadState: UploadComplete, ScanStatus: ScanClean, SetMIMEType: true, ScanEngine: true, MetricOutcome: ScanClean}
	case VerdictInfected:
		return PassOutcome{UploadState: UploadComplete, ScanStatus: ScanInfected, FailureReason: ReasonMalwareDetected,
			SetMIMEType: true, DeleteObjects: true, AuditMalware: true, ScanEngine: true, MetricOutcome: ScanInfected}
	case VerdictError:
		reason := in.Verdict.Reason
		if reason == "" {
			reason = ReasonScanEngineError
		}
		return PassOutcome{UploadState: UploadComplete, ScanStatus: ScanError, FailureReason: reason, SetMIMEType: true,
			DeleteObjects: true, ScanEngine: true, MetricOutcome: ScanError}
	}
	return PassOutcome{Retry: true}
}

// NextScanDelay is the pause before retry round+1 of a pass that failed transiently in round
// (docs/DECISIONS.md D-M3-04-1): first (file.scan.health_interval) doubled per round, never above
// ceiling (file.scan.retry_backoff_cap), without overflow for any round. A negative round counts
// as 0; a ceiling below first gives ceiling.
//
// Example:
//
//	d := domain.NextScanDelay(2, 30*time.Second, 5*time.Minute) // 2m0s
func NextScanDelay(round int64, first, ceiling time.Duration) time.Duration {
	if first <= 0 || ceiling <= first {
		return min(max(first, 0), max(ceiling, 0))
	}
	d := first
	for i := int64(0); i < round; i++ {
		if d >= ceiling/2 {
			return ceiling
		}
		d *= 2
	}
	return d
}

// AuditCodeMaxBytes is the longest code of audit_details.v1 (SEC-AUD-5); the audit module holds the
// same bound, and TestAuditSignature keeps a normalised name within it.
const AuditCodeMaxBytes = 64

// auditSignatureUnknown stands for a signature that normalises to nothing a code can start with.
const auditSignatureUnknown = "unknown"

// AuditSignature turns a ClamAV signature name into a code of audit_details.v1 (SEC-AUD-5,
// docs/DECISIONS.md D-M3-04-14): lower case, every character outside [a-z0-9_.:-] becomes "_",
// cut to AuditCodeMaxBytes. A result that does not start with a letter or digit is prefixed with
// "x", an empty one is "unknown". The original name is stored nowhere else.
//
// Example:
//
//	c := domain.AuditSignature("Win.Test.EICAR_HDB-1") // "win.test.eicar_hdb-1"
func AuditSignature(sig string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(sig) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == ':' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return auditSignatureUnknown
	}
	if c := out[0]; (c < 'a' || c > 'z') && (c < '0' || c > '9') {
		out = "x" + out
	}
	if len(out) > AuditCodeMaxBytes {
		out = out[:AuditCodeMaxBytes]
	}
	return out
}

// MediaType returns the media type of a detected type without parameters, trimmed and in lower
// case (docs/DECISIONS.md D-M3-04-6): "Text/Plain; charset=utf-8" gives "text/plain".
func MediaType(t string) string {
	if i := strings.IndexByte(t, ';'); i >= 0 {
		t = t[:i]
	}
	return strings.ToLower(strings.TrimSpace(t))
}

// TypeDenied reports whether the media type t (MediaType applied) equals one entry of denied with
// MediaType applied; empty entries never match (12 6.1, the same rule as the early check).
func TypeDenied(t string, denied []string) bool {
	t = MediaType(t)
	for _, d := range denied {
		if d = MediaType(d); d != "" && d == t {
			return true
		}
	}
	return false
}
