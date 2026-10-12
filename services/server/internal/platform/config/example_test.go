// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"fmt"
	"time"

	"github.com/OWNER/bothy/services/server/internal/platform/config"
)

func exampleSchema() *config.Schema {
	schema, err := config.NewSchema([]config.Key{
		{
			Name: "server.role", Kind: config.KindEnum, Default: "all", Enum: []string{"api", "worker", "all"},
			Doc: "Process role.", Chapter: "16-operations.md",
		},
		{
			Name: "sync.event_retention", Kind: config.KindDuration, Default: "30d", Min: "1d",
			Doc: "How long the event log keeps events.", Chapter: "04-sync-protocol.md",
		},
	})
	if err != nil {
		panic(err)
	}
	return schema
}

// Load takes the environment as an explicit list; only cmd/bothy passes os.Environ().
func ExampleLoad() {
	values, err := config.Load(exampleSchema(), config.Sources{
		Env: []string{"BOTHY_SERVER__ROLE=worker", "TZ=UTC"},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(values.String("server.role"), "from", values.Source("server.role"))
	fmt.Println(values.Text("sync.event_retention"), "from", values.Source("sync.event_retention"))

	_, err = config.Load(exampleSchema(), config.Sources{Env: []string{"BOTHY_SYNC__EVENT_RETENTION=12h"}})
	fmt.Println(err)
	// Output:
	// worker from env BOTHY_SERVER__ROLE
	// 30d from default
	// config: sync.event_retention: must be at least 1d (source: env BOTHY_SYNC__EVENT_RETENTION)
}

// A consumer declares its Config in its own package; internal/wire fills it with Bind.
func ExampleBind() {
	type SyncConfig struct {
		Role      string        `config:"server.role"`
		Retention time.Duration `config:"sync.event_retention"`
	}
	values, err := config.Load(exampleSchema(), config.Sources{})
	if err != nil {
		fmt.Println(err)
		return
	}
	var c SyncConfig
	if err := config.Bind(values, &c); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(c.Role, c.Retention)
	// Output: all 720h0m0s
}
