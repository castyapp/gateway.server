package tests

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/castyapp/gateway.server/config"
)

var defaultConfig = &config.ConfMap{
	Debug: false,
	Env:   "dev",
	Redis: config.RedisConfig{
		Cluster:    false,
		MasterName: "casty",
		Addr:       "127.0.0.1:26379",
		Pass:       "super-secure-password",
		Sentinels: []string{
			"127.0.0.1:26379",
		},
		SentinelPass: "super-secure-sentinels-password",
	},
	Grpc: config.GrpcConfig{
		Host: "localhost",
		Port: 55283,
	},
	Sentry: config.SentryConfig{
		Enabled: false,
		Dsn:     "sentry.dsn.here",
	},
}

func TestLoadConfig(t *testing.T) {
	if err := config.LoadFile(filepath.Join("./config_test.hcl")); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !reflect.DeepEqual(defaultConfig, config.Map) {
		t.Fatalf("bad: %#v", config.Map)
	}
}
