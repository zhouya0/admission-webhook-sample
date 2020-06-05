package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"context"
	"flag"
	"github.com/zhouya0/admission-webhook-sample/pkg"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog"
)


func main() {
	var parameters pkg.WhSvrParameters

	// get command line parameters
	flag.IntVar(&parameters.Port, "port", 443, "Webhook server port.")
	flag.StringVar(&parameters.CertFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.KeyFile, "tlsKeyFile", "/etc/webhook/cert/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	pair, err := tls.LoadX509KeyPair(parameters.CertFile, parameters.KeyFile)
	if err != nil {
		klog.Errorf("Failed to load key pair: %v", err)
	}

	whsvr := &pkg.WebhookServer{
		Server: &http.Server{
			Addr: fmt.Sprintf(":%v", parameters),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}},
		},
	}

	// define http server and handler
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", whsvr.Serve)

	// start webhook server in new routine
	go func() {
		if err := whsvr.Server.ListenAndServeTLS("", ""); err != nil {
			klog.Errorf("Failed to listen and serve webhook server: %v", err)
		}
	}()

	klog.Info("Server started ")

	// listening OS shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	klog.Infof("Got OS shutdown signal, shutting down webhook server gracefullt...")
	whsvr.Server.Shutdown(context.Background())
}
