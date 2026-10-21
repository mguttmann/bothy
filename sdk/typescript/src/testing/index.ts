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
