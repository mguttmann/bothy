// SPDX-License-Identifier: Apache-2.0

package app_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/test/appsworld"
)

// The webhook scenarios of M2-26 measured in the package app (test/appsworld).

// TestWebhookRoundCreatesDeliveries runs appsworld.ScenarioWebhookRoundCreatesDeliveries.
func TestWebhookRoundCreatesDeliveries(t *testing.T) {
	appsworld.ScenarioWebhookRoundCreatesDeliveries(t)
}

// TestWebhookDeliveryG10 runs appsworld.ScenarioWebhookDeliveryG10.
func TestWebhookDeliveryG10(t *testing.T) { appsworld.ScenarioWebhookDeliveryG10(t) }

// TestAZM49 runs appsworld.ScenarioAZM49.
func TestAZM49(t *testing.T) { appsworld.ScenarioAZM49(t) }

// TestDeliveryRevokedRightIsDeadWithoutRequest runs appsworld.ScenarioDeliveryRevokedRightIsDeadWithoutRequest.
func TestDeliveryRevokedRightIsDeadWithoutRequest(t *testing.T) {
	appsworld.ScenarioDeliveryRevokedRightIsDeadWithoutRequest(t)
}

// TestDeliveryScopeRemovedIsDead runs appsworld.ScenarioDeliveryScopeRemovedIsDead.
func TestDeliveryScopeRemovedIsDead(t *testing.T) { appsworld.ScenarioDeliveryScopeRemovedIsDead(t) }

// TestDeliveryStrictOrder runs appsworld.ScenarioDeliveryStrictOrder.
func TestDeliveryStrictOrder(t *testing.T) { appsworld.ScenarioDeliveryStrictOrder(t) }

// TestDelivery410Disables runs appsworld.ScenarioDelivery410Disables.
func TestDelivery410Disables(t *testing.T) { appsworld.ScenarioDelivery410Disables(t) }

// TestDeliveryRetryAfter runs appsworld.ScenarioDeliveryRetryAfter.
func TestDeliveryRetryAfter(t *testing.T) { appsworld.ScenarioDeliveryRetryAfter(t) }

// TestDeliveryBackoffAndConsecutiveFailures runs appsworld.ScenarioDeliveryBackoffAndConsecutiveFailures.
func TestDeliveryBackoffAndConsecutiveFailures(t *testing.T) {
	appsworld.ScenarioDeliveryBackoffAndConsecutiveFailures(t)
}

// TestDeliveryDeadAfterRetryMaxAge runs appsworld.ScenarioDeliveryDeadAfterRetryMaxAge.
func TestDeliveryDeadAfterRetryMaxAge(t *testing.T) {
	appsworld.ScenarioDeliveryDeadAfterRetryMaxAge(t)
}

// TestDeliveryPrivateAddressRefused runs appsworld.ScenarioDeliveryPrivateAddressRefused.
func TestDeliveryPrivateAddressRefused(t *testing.T) {
	appsworld.ScenarioDeliveryPrivateAddressRefused(t)
}

// TestDeliveryGapDisables runs appsworld.ScenarioDeliveryGapDisables.
func TestDeliveryGapDisables(t *testing.T) { appsworld.ScenarioDeliveryGapDisables(t) }

// TestConsumerLossDisablesSubscriptions runs appsworld.ScenarioConsumerLossDisablesSubscriptions.
func TestConsumerLossDisablesSubscriptions(t *testing.T) {
	appsworld.ScenarioConsumerLossDisablesSubscriptions(t)
}

// TestDeliveryQuotaPauses runs appsworld.ScenarioDeliveryQuotaPauses.
func TestDeliveryQuotaPauses(t *testing.T) { appsworld.ScenarioDeliveryQuotaPauses(t) }

// TestDeliverySuspendedInstallation runs appsworld.ScenarioDeliverySuspendedInstallation.
func TestDeliverySuspendedInstallation(t *testing.T) {
	appsworld.ScenarioDeliverySuspendedInstallation(t)
}

// TestInstallCreatesManifestSubscription runs appsworld.ScenarioInstallCreatesManifestSubscription.
func TestInstallCreatesManifestSubscription(t *testing.T) {
	appsworld.ScenarioInstallCreatesManifestSubscription(t)
}

// TestUninstallDisablesSubscriptions runs appsworld.ScenarioUninstallDisablesSubscriptions.
func TestUninstallDisablesSubscriptions(t *testing.T) {
	appsworld.ScenarioUninstallDisablesSubscriptions(t)
}

// TestDeliveryConcurrentJobs runs appsworld.ScenarioDeliveryConcurrentJobs.
func TestDeliveryConcurrentJobs(t *testing.T) { appsworld.ScenarioDeliveryConcurrentJobs(t) }

// TestDeliveryCrashAfterSend runs appsworld.ScenarioDeliveryCrashAfterSend.
func TestDeliveryCrashAfterSend(t *testing.T) { appsworld.ScenarioDeliveryCrashAfterSend(t) }

// TestDeliveryDeletedDuringAttempt runs appsworld.ScenarioDeliveryDeletedDuringAttempt.
func TestDeliveryDeletedDuringAttempt(t *testing.T) {
	appsworld.ScenarioDeliveryDeletedDuringAttempt(t)
}

// TestWebhookSecretsNeverLeak runs appsworld.ScenarioWebhookSecretsNeverLeak.
func TestWebhookSecretsNeverLeak(t *testing.T) { appsworld.ScenarioWebhookSecretsNeverLeak(t) }

// TestDeleteWebhookSubscription runs appsworld.ScenarioDeleteWebhookSubscription.
func TestDeleteWebhookSubscription(t *testing.T) { appsworld.ScenarioDeleteWebhookSubscription(t) }

// TestListWebhookSubscriptionsAndDeliveries runs appsworld.ScenarioListWebhookSubscriptionsAndDeliveries.
func TestListWebhookSubscriptionsAndDeliveries(t *testing.T) {
	appsworld.ScenarioListWebhookSubscriptionsAndDeliveries(t)
}

// TestRedeliverWebhook runs appsworld.ScenarioRedeliverWebhook.
func TestRedeliverWebhook(t *testing.T) { appsworld.ScenarioRedeliverWebhook(t) }

// TestUpdateWebhookSubscription runs appsworld.ScenarioUpdateWebhookSubscription.
func TestUpdateWebhookSubscription(t *testing.T) { appsworld.ScenarioUpdateWebhookSubscription(t) }
