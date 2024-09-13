package main

import (
	"crypto/tls"
	"log"
	"net/http"

	"github.com/bentito/clusterextensionhelper/pkg/webhook"
)

func main() {
	// Load TLS certificates (you need to generate these and mount them into the container)
	cert, err := tls.LoadX509KeyPair("/tls/tls.crt", "/tls/tls.key")
	if err != nil {
		log.Fatalf("Failed to load key pair: %v", err)
	}

	server := &http.Server{
		Addr: ":8443",
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}

	http.HandleFunc("/mutate", webhook.Mutate)

	log.Println("Starting webhook server...")
	if err := server.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
