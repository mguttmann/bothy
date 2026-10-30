// SPDX-License-Identifier: Apache-2.0
/**
 * Testing support of the SDK (`bothy-sdk/testing`): FakeServer, `signWebhook`, the shared
 * scenario functions and their doubles. Node.js only.
 *
 * @packageDocumentation
 */

export { FakeClock, settle } from "./clock.js";
export {
  FORGED_VARIANTS,
  type ForgedVariant,
  type RunView,
  type ScenarioServer,
  type Triggered,
} from "./scenario-server.js";
export {
  FakeServer,
  TIMING_KEYS,
  type ConsumerView,
  type FakeServerOptions,
  type RecordedFrame,
  type RecordedRequest,
  type SendWebhookOptions,
  type TimingKey,
  type Timings,
} from "./fakeserver.js";
export { Fault, FaultKind } from "./faults.js";
export {
  signWebhook,
  webhookPublicKey,
  type SignWebhookOptions,
} from "./webhooks.js";
export {
  KEEP,
  REMOVE,
  RecordingHandler,
  RecordingSource,
  TamperingSource,
  fingerprint,
  type RecordingHandlerOptions,
  type Rewrite,
} from "./doubles.js";
export {
  ScenarioFailed,
  ag08InvalidTokens,
  ag09DoubleDelivery,
  ag12PatchRules,
  ag28SlowStartAndRestart,
  azp6AdapterNeverStarts,
  rag19AllowlistFromServer,
} from "./scenarios.js";
