package controllers

import "database/sql"

type LimitExceededError struct {
	Message string
}

func (e *LimitExceededError) Error() string { return e.Message }

var ProjectLimitHook func(tx *sql.Tx, orgId int) error

var MemberLimitHook func(tx *sql.Tx, orgId int) error

// CheckLimitHook caps synthetic checks per organization (cloud plans). Nil
// means unlimited; a LimitExceededError surfaces as a 422 in the dialog.
var CheckLimitHook func(tx *sql.Tx, orgId int) error
