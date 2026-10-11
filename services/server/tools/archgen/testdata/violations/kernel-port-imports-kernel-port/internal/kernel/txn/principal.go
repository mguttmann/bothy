// SPDX-License-Identifier: Apache-2.0

package txn

import "example.com/skeleton/internal/kernel/actor"

// Owner ties a unit of work to a principal; kernel ports do not use each other.
type Owner actor.Principal
