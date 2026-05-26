package main

import (
	"fmt"
	"log"

	"secpay/config"
)

func main() {
	log.Println("Starting financial application...")

	// Load configuration from root path "." (where config.yaml is located)
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Println("==================================================")
	fmt.Println("           APPLICATION LAUNCHED SUCCESSFULLY       ")
	fmt.Println("==================================================")
	fmt.Printf("Environment : %s\n", cfg.AppEnv)
	fmt.Printf("Server Port : %s\n", cfg.Port)
	fmt.Printf("Database    : %s:%s/%s (User: %s)\n", cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBUser)
	fmt.Println("==================================================")

	log.Println("Domain layer and Configuration management verified.")
}
