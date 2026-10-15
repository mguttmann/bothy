// SPDX-License-Identifier: Apache-2.0

package httpapi_test

import (
	"testing"

	"github.com/OWNER/bothy/services/server/test/appsworld"
)

// The webhook API scenarios of M2-26 measured in the package httpapi (test/appsworld).

// TestCreateWebhookSubscription runs appsworld.ScenarioCreateWebhookSubscription.
func TestCreateWebhookSubscription(t *testing.T) { appsworld.ScenarioCreateWebhookSubscription(t) }

// TestUpdateWebhookSubscription runs appsworld.ScenarioUpdateWebhookSubscription.
func TestUpdateWebhookSubscription(t *testing.T) { appsworld.ScenarioUpdateWebhookSubscription(t) }

// TestDeleteWebhookSubscription runs appsworld.ScenarioDeleteWebhookSubscription.
func TestDeleteWebhookSubscription(t *testing.T) { appsworld.ScenarioDeleteWebhookSubscription(t) }

// TestListWebhookSubscriptionsAndDeliveries runs appsworld.ScenarioListWebhookSubscriptionsAndDeliveries.
func TestListWebhookSubscriptionsAndDeliveries(t *testing.T) {
	appsworld.ScenarioListWebhookSubscriptionsAndDeliveries(t)
}

// TestGetWebhookKeys runs appsworld.ScenarioGetWebhookKeys.
func TestGetWebhookKeys(t *testing.T) { appsworld.ScenarioGetWebhookKeys(t) }

// TestRedeliverWebhook runs appsworld.ScenarioRedeliverWebhook.
func TestRedeliverWebhook(t *testing.T) { appsworld.ScenarioRedeliverWebhook(t) }
