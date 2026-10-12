// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/OWNER/bothy/services/server/internal/platform/otel/attr"
)

// Resource attributes of signals.md "Resource attributes".
const (
	resServiceName       = "service.name"
	resServiceNamespace  = "service.namespace"
	resServiceVersion    = "service.version"
	resServiceInstanceID = "service.instance.id"
	resEnvironment       = "deployment.environment.name"

	serviceName      = "bothy-server"
	serviceNamespace = "bothy"
)

// newResource builds the resource of the process (D2): the attributes of
// OTEL_RESOURCE_ATTRIBUTES and the SDK attributes first, then the attributes of the process,
// which win. deployment.environment.name comes from OTEL_RESOURCE_ATTRIBUTES when it is set
// there, else from instance.environment. started is the start time that makes
// service.instance.id unique per process next to the host name.
func newResource(ctx context.Context, cfg Config, version string, started time.Time) (*resource.Resource, error) {
	fromEnv, err := resource.New(ctx, resource.WithFromEnv(), resource.WithTelemetrySDK())
	if err != nil {
		// The error of package resource quotes the value; the message names the variable only.
		return nil, fmt.Errorf("%w OTEL_RESOURCE_ATTRIBUTES: want comma-separated key=value pairs", ErrInvalidEnv)
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	own := []attribute.KeyValue{
		attribute.String(resServiceName, serviceName),
		attribute.String(resServiceNamespace, serviceNamespace),
		attribute.String(resServiceInstanceID, host+"-"+strconv.FormatInt(started.UnixNano(), 10)),
		attribute.String(attr.ServerRole, cfg.Role),
	}
	if version != "" {
		own = append(own, attribute.String(resServiceVersion, version))
	}
	if _, set := fromEnv.Set().Value(resEnvironment); !set && cfg.Environment != "" {
		own = append(own, attribute.String(resEnvironment, cfg.Environment))
	}
	return resource.Merge(fromEnv, resource.NewSchemaless(own...))
}
