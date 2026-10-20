// SPDX-License-Identifier: Apache-2.0

// Package domain holds the message rules of the messaging module that need no I/O (M1-56): the
// message row, its origin from the principal kind (09 section 3.2), the content and client_msg_id
// checks, the mention scanner of content format 1 (03 messages "Inhaltsformat") with the code
// structure of CommonMark, the canonical request hash (04 section 5.1), the system line of a
// reopen and the cut of a content for an event payload (04 payload rule 2).
package domain
