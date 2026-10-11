// SPDX-License-Identifier: Apache-2.0

// Package postgres is the database adapter of the chat module.
package postgres

import (
	"github.com/jackc/pgx/v5"

	"example.com/skeleton/internal/chat"
	"example.com/skeleton/internal/chat/app"
	"example.com/skeleton/internal/chat/domain"
	"example.com/skeleton/internal/platform/pg"
)

// Repository stores channels.
type Repository struct {
	Pool    *pg.Pool
	Conn    *pgx.Conn
	Service *app.Service
	Last    domain.Channel
	Port    chat.Channel
}
