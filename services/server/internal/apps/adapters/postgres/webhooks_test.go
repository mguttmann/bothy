// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/test/appsworld"
)

// The webhook scenarios of M2-26 that exercise the store, measured in the package postgres (test/appsworld).

// TestWebhookRoundCreatesDeliveries runs appsworld.ScenarioWebhookRoundCreatesDeliveries.
func TestWebhookRoundCreatesDeliveries(t *testing.T) {
	appsworld.ScenarioWebhookRoundCreatesDeliveries(t)
}

// TestDeliveryQuotaPauses runs appsworld.ScenarioDeliveryQuotaPauses.
func TestDeliveryQuotaPauses(t *testing.T) { appsworld.ScenarioDeliveryQuotaPauses(t) }

// TestConsumerLossDisablesSubscriptions runs appsworld.ScenarioConsumerLossDisablesSubscriptions.
func TestConsumerLossDisablesSubscriptions(t *testing.T) {
	appsworld.ScenarioConsumerLossDisablesSubscriptions(t)
}

// TestDeliveryGapDisables runs appsworld.ScenarioDeliveryGapDisables.
func TestDeliveryGapDisables(t *testing.T) { appsworld.ScenarioDeliveryGapDisables(t) }

// TestRedeliverWebhook runs appsworld.ScenarioRedeliverWebhook.
func TestRedeliverWebhook(t *testing.T) { appsworld.ScenarioRedeliverWebhook(t) }

// TestListWebhookSubscriptionsAndDeliveries runs appsworld.ScenarioListWebhookSubscriptionsAndDeliveries.
func TestListWebhookSubscriptionsAndDeliveries(t *testing.T) {
	appsworld.ScenarioListWebhookSubscriptionsAndDeliveries(t)
}

// TestDeleteWebhookSubscription runs appsworld.ScenarioDeleteWebhookSubscription.
func TestDeleteWebhookSubscription(t *testing.T) { appsworld.ScenarioDeleteWebhookSubscription(t) }

// TestUninstallDisablesSubscriptions runs appsworld.ScenarioUninstallDisablesSubscriptions.
func TestUninstallDisablesSubscriptions(t *testing.T) {
	appsworld.ScenarioUninstallDisablesSubscriptions(t)
}

// TestDeliverySuspendedInstallation runs appsworld.ScenarioDeliverySuspendedInstallation.
func TestDeliverySuspendedInstallation(t *testing.T) {
	appsworld.ScenarioDeliverySuspendedInstallation(t)
}

// TestInstallCreatesManifestSubscription runs appsworld.ScenarioInstallCreatesManifestSubscription.
func TestInstallCreatesManifestSubscription(t *testing.T) {
	appsworld.ScenarioInstallCreatesManifestSubscription(t)
}

// TestUpdateWebhookSubscription runs appsworld.ScenarioUpdateWebhookSubscription.
func TestUpdateWebhookSubscription(t *testing.T) { appsworld.ScenarioUpdateWebhookSubscription(t) }

// TestCreateWebhookSubscription runs appsworld.ScenarioCreateWebhookSubscription.
func TestCreateWebhookSubscription(t *testing.T) { appsworld.ScenarioCreateWebhookSubscription(t) }

// TestAZM49 runs appsworld.ScenarioAZM49.
func TestAZM49(t *testing.T) { appsworld.ScenarioAZM49(t) }
