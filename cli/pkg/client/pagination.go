package client

import "github.com/tracewayapp/traceway/cli/pkg/access"

// PaginationParams is the request-side pagination control.
type PaginationParams = access.PageRequest

// Pagination is the response-side pagination block. Traceway returns this
// on every paginated list endpoint.
type Pagination = access.Pagination
