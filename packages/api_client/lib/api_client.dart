// SPDX-License-Identifier: Apache-2.0

/// REST client of the chat server: the API generated from `docs/api/openapi.yaml`
/// (openapi-generator `dart-dio`, never changed by hand) and the hand-written layer every
/// client package uses (docs/spec/06-api.md 14).
///
/// Build a client with [createApiClient]; it sets `traceparent` on every request and
/// `Idempotency-Key` on writes from [idempotencyKeyExtra], retries reads only
/// ([ReadRetryPolicy]), maps problems to [ApiProblem] and offers the interceptor slot for the
/// authentication of the `auth` package. Read events with [EventsPageDecoder] or
/// [fetchEvents], which skip unknown event types instead of failing.
library;

export 'src/generated/generated_api.dart';
export 'src/hand/client_factory.dart';
export 'src/hand/events.dart';
export 'src/hand/headers.dart';
export 'src/hand/problem.dart';
export 'src/hand/read_retry.dart';
