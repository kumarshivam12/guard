package webhook

import (
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/component-base/cli/globalflag"
)

// Options Setting Up the HTTPS server For Request and Response
type Options struct {
	SecureServingOptions options.SecureServingOptions
}

// AddFlagSet Adding Flag Support
func (o *Options) AddFlagSet(fs *pflag.FlagSet) {
	o.SecureServingOptions.AddFlags(fs)
}

type Config struct {
	SecureServingInfo *server.SecureServingInfo
}

const (
	valkontroller = "val-kontroller"
)

func (o *Options) Config() *Config {
	if err := o.SecureServingOptions.MaybeDefaultWithSelfSignedCerts("0.0.0.0", nil, nil); err != nil {
		sentry.CaptureException(err)
		panic(err)
	}

	c := Config{}
	if err := o.SecureServingOptions.ApplyTo(&c.SecureServingInfo); err != nil {
		sentry.CaptureException(err)
		panic(err)
	}
	return &c
}

func DefaultServerOptions() *Options {
	NewOption := &Options{
		SecureServingOptions: *options.NewSecureServingOptions(),
	}
	NewOption.SecureServingOptions.BindPort = 8443
	NewOption.SecureServingOptions.ServerCert.PairName = valkontroller
	return NewOption
}

var valkontrollerCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "No of request handled by validation handler function",
	},
	[]string{"code", "status"},
)

var rejectionCounter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "rejected_requests_total",
		Help: "No of request Rejected by validation handler function",
	},
	[]string{"namespace", "deployment"},
)

// Init Starting HTTPS Server
func Init() {
	prometheus.MustRegister(valkontrollerCounter)
	prometheus.MustRegister(rejectionCounter)
	option := DefaultServerOptions()
	fs := pflag.NewFlagSet(valkontroller, pflag.ExitOnError)
	globalflag.AddGlobalFlags(fs, valkontroller)
	option.AddFlagSet(fs)
	sentrylog()
	if err := fs.Parse(os.Args); err != nil {
		sentry.CaptureException(err)
		panic(err)
	}
	c := option.Config()

	mux := http.NewServeMux()
	mux.Handle("/validate", http.HandlerFunc(ValidatePod))
	mux.Handle("/metrics", promhttp.Handler())
	// This Channel will Run Until Gets SIGTERM or SIGINT
	stopCh := server.SetupSignalHandler()
	ch, err := c.SecureServingInfo.Serve(mux, 60*time.Second, stopCh)
	if err != nil {
		sentry.CaptureException(err)
		panic(err)
	} else {
		<-ch
	}
}
