package tracewaybackend

import "github.com/tracewayapp/traceway/backend/cmd"

type Option = cmd.Option

var (
	Run                = cmd.Run
	WithPort           = cmd.WithPort
	WithServerURL      = cmd.WithServerURL
	WithSQLitePath     = cmd.WithSQLitePath
	WithDefaultUser    = cmd.WithDefaultUser
	WithDefaultProject = cmd.WithDefaultProject
	WithMonitoringURL  = cmd.WithMonitoringURL
	DisableLogging     = cmd.DisableLogging

	WithDefaultProjectSourceMapToken = cmd.WithDefaultProjectSourceMapToken
)
