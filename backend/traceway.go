package tracewaybackend

import "github.com/tracewayapp/traceway/backend/v2/cmd"

type Option = cmd.Option

var (
	Run                = cmd.Run
	Shutdown           = cmd.Shutdown
	ShutdownOnSignal   = cmd.ShutdownOnSignal
	WithPort           = cmd.WithPort
	WithServerURL      = cmd.WithServerURL
	WithSQLitePath     = cmd.WithSQLitePath
	WithDefaultUser    = cmd.WithDefaultUser
	WithDefaultProject = cmd.WithDefaultProject
	WithMonitoringURL  = cmd.WithMonitoringURL
	WithJWTSecret      = cmd.WithJWTSecret
	DisableLogging     = cmd.DisableLogging

	WithDefaultProjectSourceMapToken = cmd.WithDefaultProjectSourceMapToken
)
