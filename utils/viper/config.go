package utils

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func UpdateViperConfig(key string, value any, configFile string) error {
	viper.Set(key, value)

	updatedConfig, err := yaml.Marshal(viper.AllSettings())
	if err != nil {
		fmt.Printf("unable to marshal config to yaml: %v\n", err)
	}

	err = os.WriteFile(configFile, updatedConfig, 0o644)
	if err != nil {
		fmt.Printf("failed to update config file: %v\n", err)
	}

	return nil
}
