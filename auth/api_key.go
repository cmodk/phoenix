package auth

import (
	"time"
)

type ApiKey struct {
	Id             uint64    `db:"id"`
	Token          string    `db:"token" json:"-"`
	ExpirationTime time.Time `db:"expiration_time"`
	UserId         uint64    `db:"user_id"`
}
