package config

import (
	"io/ioutil"

	"github.com/hashicorp/hcl"
)

type ConfMap struct {
	Debug  bool         `hcl:"debug"`
	Env    string       `hcl:"env"`
	Grpc   GrpcConfig   `hcl:"grpc,block"`
	Redis  RedisConfig  `hcl:"redis,block"`
	Sentry SentryConfig `hcl:"sentry,block"`
}

type SentryConfig struct {
	Enabled bool   `hcl:"enabled"`
	Dsn     string `hcl:"dsn"`
}

type RedisConfig struct {
	Cluster      bool     `hcl:"cluster"`
	MasterName   string   `hcl:"master_name"`
	Addr         string   `hcl:"addr"`
	Sentinels    []string `hcl:"sentinels"`
	Pass         string   `hcl:"pass"`
	SentinelPass string   `hcl:"sentinel_pass"`
}

type GrpcConfig struct {
	Host string `hcl:"host"`
	Port int    `hcl:"port"`
}

var Map = new(ConfMap)

func LoadFile(filename string) (err error) {

	d, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	obj, err := hcl.Parse(string(d))
	if err != nil {
		return err
	}

	// Build up the result
	if err := hcl.DecodeObject(&Map, obj); err != nil {
		return err
	}

	return
}
