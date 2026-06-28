package controllers

import "database/sql"

type LimitExceededError struct {
	Message string
}

func (e *LimitExceededError) Error() string { return e.Message }

var ProjectLimitHook func(tx *sql.Tx, orgId int) error

var MemberLimitHook func(tx *sql.Tx, orgId int) error
