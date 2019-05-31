package config

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

// AppConfig - holds configuration details for the App
type AppConfig struct {
	Logging  LoggingConfig  `yaml:"Logging"`
	Newrelic NewrelicConfig `yaml:"Newrelic"`
	Service  ServiceConfig  `yaml:"Service"`
}

// LoggingConfig - holds configuration details for the logging lib
type LoggingConfig struct {
	AppName    string `yaml:"AppName"`
	AppVersion string `yaml:"AppVersion"`
	EngGroup   string `yaml:"EngGroup"`
	Level      string `yaml:"Level"`
}

// ServiceConfig - contains all the service details for service
type ServiceConfig struct {
	Listen int `yaml:"Listen"`
}

// NewrelicConfig - contains all key details for invoking Newrelic go sdk
type NewrelicConfig struct {
	License string `yaml:"License"`
	Name    string `yaml:"Name"`
}

// LoadConfiguration - read configurations from a yaml file and loads into 'Config' struct.
//					   returns error if the file is missing or contains bad schema.
func LoadConfiguration(filePath string) (*AppConfig, error) {
	cnf := &AppConfig{}

	rawContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read configuration file %s", err.Error())
	}

	ymlErr := yaml.Unmarshal(rawContent, &cnf)
	if ymlErr != nil {
		return nil, fmt.Errorf("unable to unpack file %s", ymlErr.Error())
	}

	return cnf, nil
}
