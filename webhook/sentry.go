package webhook

import (
	"log"
	"time"

	"github.com/certifi/gocertifi"
	"github.com/getsentry/sentry-go"
)

func sentrylog() {
	// struct type with sentry option
	sentryClientOptions := sentry.ClientOptions{
		Dsn:         "",
		Environment: "Production",
		Release:     "validation-kontroller@0.0.1",
		Debug:       true,
	}
	rootCAs, err := gocertifi.CACerts()
	if err != nil {
		log.Printf("Could not load CA Certificates: %v\n", err)
	} else {
		sentryClientOptions.CaCerts = rootCAs
	}
	sentry.Init(sentryClientOptions)
	// Flush buffered events before the program terminates.
	defer sentry.Flush(2 * time.Second)
}
