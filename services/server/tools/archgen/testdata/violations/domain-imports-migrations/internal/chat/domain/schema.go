// SPDX-License-Identifier: Apache-2.0

package domain

import "example.com/skeleton/migrations"

// Version reads the migrations, which only cmd, wire and the platform packages in used_by may do.
const Version = migrations.Latest
