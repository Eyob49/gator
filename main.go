package main

import (
	"fmt"
	"log"

	"github.com/Eyob49/gator/internal/config"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	if err := config.SetUser("Eyob"); err != nil {
		log.Fatalf("Error setting user: %v", err)
	}

	cfg, err = config.Read()
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}

	fmt.Printf("Config: %+v\n", cfg)
}
