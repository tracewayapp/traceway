package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
)

// Embedded mode runs inside the host application's process, where JWT_SECRET
// usually belongs to the host app. Only Traceway-specific sources are read.
func embeddedJWTSecret(o *options) string {
	if o.jwtSecret != nil {
		return *o.jwtSecret
	}
	if secret, ok := os.LookupEnv("TRACEWAY_JWT_SECRET"); ok {
		return secret
	}
	if !o.disableLogging {
		log.Println("[tracewaybackend] No JWT secret configured, using a random one for this run. Dashboard sessions will not survive a restart. Pass tracewaybackend.WithJWTSecret(secret) (32+ characters) or set TRACEWAY_JWT_SECRET to keep them.")
	}
	secret := make([]byte, 32)
	rand.Read(secret)
	return hex.EncodeToString(secret)
}

type options struct {
	sqlitePath            string
	port                  int
	serverURL             string
	disableLogging        bool
	defaultUser           *defaultUserOpts
	defaultProjects       []defaultProjectOpts
	monitoringTracewayURL string
	jwtSecret             *string
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

func WithJWTSecret(secret string) Option {
	return func(o *options) {
		o.jwtSecret = &secret
	}
}
