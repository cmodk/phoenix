package auth

import (
	"time"
)

type User struct {
	Id        uint64     `db:"id" json:"id"`
	Email     string     `db:"email" json:"email"`
	AuthType  string     `db:"auth_type" json:"email"`
	LastLogin *time.Time `db:"last_login" json:"last_login"`
}
