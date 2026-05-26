package config

import (
	"log"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	AppEnv  string `mapstructure:"APP_ENV"`
	Port    string `mapstructure:"PORT"`
	DBHost  string `mapstructure:"DB_HOST"`
	DBPort  string `mapstructure:"DB_PORT"`
	DBUser  string `mapstructure:"DB_USER"`
	DBPass  string `mapstructure:"DB_PASS"`
	DBName  string `mapstructure:"DB_NAME"`
}

// LoadConfig reads configuration from file or environment variables.
func LoadConfig(path string) (*Config, error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Allow environment variables to override file values
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Could not read config file: %v. Falling back to environment variables.", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
