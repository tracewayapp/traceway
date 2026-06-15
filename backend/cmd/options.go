package cmd

type options struct {
	sqlitePath            string
	port                  int
	serverURL             string
	disableLogging        bool
	defaultUser           *defaultUserOpts
	defaultProjects       []defaultProjectOpts
	monitoringTracewayURL string
}

type defaultUserOpts struct {
	email    string
	password string
}

type defaultProjectOpts struct {
	name           string
	framework      string
	token          string
	sourceMapToken string
}

type Option func(*options)

func WithSQLitePath(path string) Option {
	return func(o *options) {
		o.sqlitePath = path
	}
}

func WithPort(port int) Option {
	return func(o *options) {
		o.port = port
	}
}

func WithServerURL(url string) Option {
	return func(o *options) {
		o.serverURL = url
	}
}

func WithDefaultUser(email, password string) Option {
	return func(o *options) {
		o.defaultUser = &defaultUserOpts{email: email, password: password}
	}
}

func DisableLogging() Option {
	return func(o *options) {
		o.disableLogging = true
	}
}

func WithDefaultProject(name, framework, token string) Option {
	return func(o *options) {
		o.defaultProjects = append(o.defaultProjects, defaultProjectOpts{
			name: name, framework: framework, token: token,
		})
	}
}

func WithDefaultProjectSourceMapToken(name, token string) Option {
	return func(o *options) {
		for i := range o.defaultProjects {
			if o.defaultProjects[i].name == name {
				o.defaultProjects[i].sourceMapToken = token
			}
		}
	}
}

func WithMonitoringURL(url string) Option {
	return func(o *options) {
		o.monitoringTracewayURL = url
	}
}
