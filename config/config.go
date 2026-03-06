package config

import (
	"github.com/nojyerac/go-lib/config"
	"github.com/nojyerac/go-lib/db"
	"github.com/nojyerac/go-lib/health"
	"github.com/nojyerac/go-lib/log"
	"github.com/nojyerac/go-lib/tracing"
	"github.com/nojyerac/go-lib/transport"
	"github.com/nojyerac/go-lib/transport/http"
	"github.com/nojyerac/go-lib/version"
)

var (
	DBConfig     *db.Configuration
	HealthConfig *health.Configuration
	HTTPConfig   *http.Configuration
	LogConfig    *log.Configuration
	TraceConfig  *tracing.Configuration
	TransConfig  *transport.Configuration
)

// InitAndValidate initializes the config from environment variables and validates it.
func InitAndValidate() error {
	loader := config.NewConfigLoader(version.GetVersion().Name)
	DBConfig = db.NewConfiguration()
	if err := loader.RegisterConfig(DBConfig); err != nil {
		return err
	}
	LogConfig = log.NewConfiguration()
	if err := loader.RegisterConfig(LogConfig); err != nil {
		return err
	}
	HealthConfig = health.NewConfiguration()
	if err := loader.RegisterConfig(HealthConfig); err != nil {
		return err
	}
	HTTPConfig = http.NewConfiguration()
	if err := loader.RegisterConfig(HTTPConfig); err != nil {
		return err
	}
	TransConfig = transport.NewConfiguration()
	if err := loader.RegisterConfig(TransConfig); err != nil {
		return err
	}
	TraceConfig = &tracing.Configuration{}
	if err := loader.RegisterConfig(TraceConfig); err != nil {
		return err
	}

	if err := loader.InitAndValidate(); err != nil {
		return err
	}
	return nil
}
