package main

import (
	"log"

	"github.com/SocialRoots/sr-email/pkg/web"
	"github.com/SocialRoots/sr-email/settings"
)

func main() {
	server := web.NewServer(web.ServerConfig{Env: settings.Env})
	engine := server.GinEngine()

	log.Printf("Starting sr-email on port %s [env=%s]", settings.ApiPort, settings.Env)
	if err := engine.Run(":" + settings.ApiPort); err != nil {
		log.Panicf("engine.Run: %v", err)
	}
}