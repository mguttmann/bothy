// SPDX-License-Identifier: Apache-2.0

// Package app holds the use cases of the messaging module (M1-56): sending a message into a thread
// (createMessage) and creating a thread with its first message (createThread), each in one unit
// of work with the idempotency of 04 section 5.1, the thread sequence and reopen of chat, the
// mention resolution of 07 section 4.6, the in-transaction event MessagePosted and the durable
// event message.created.
package app
