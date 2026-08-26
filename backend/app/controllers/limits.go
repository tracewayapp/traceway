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

// OrganizationLimitHook caps how many organizations a single user may own
// (cloud plans). Unlike the hooks above it is keyed on the user, not an
// organization, because it runs before the organization exists. Nil means
// unlimited; a LimitExceededError surfaces as a 422 in the dialog.
var OrganizationLimitHook func(tx *sql.Tx, userId int) error
