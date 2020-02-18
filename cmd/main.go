package main

import (
	"flag"
	"fmt"
	"net/http"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cc "github.com/arutselvan15/estore-common/clients"
	gc "github.com/arutselvan15/estore-common/config"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
	"github.com/arutselvan15/estore-product-kube-webhook/webhook"
)

func main() {
	var (
		port, certFile, keyFile string
		config                  *rest.Config
		err                     error
	)

	flag.StringVar(&port, "port", "8000", "Webhook server port.")
	flag.StringVar(&certFile, "tlsCertFile", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&keyFile, "tlsKeyFile", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	// kube config defined in env
	if gc.GetKubeConfigPath() != "" {
		config, err = clientcmd.BuildConfigFromFlags("", gc.GetKubeConfigPath())
		if err != nil {
			panic(fmt.Sprintf("error creating config using kube config path: %v", err.Error()))
		}
	} else {
		// default get current cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(fmt.Sprintf("error creating config using cluster config: %v", err))
		}
	}

	// create client for config
	estoreClients, err := cc.NewEstoreClientForConfig(config)
	if err != nil {
		panic(fmt.Sprintf("error creating clients: %v", err))
	}

	// web hook server
	whsvr := webhook.Server{
		Clients: estoreClients,
	}

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.MutateURL, whsvr.Serve)
	mux.HandleFunc(cfg.ValidateURL, whsvr.Serve)

	server := &http.Server{
		// We listen on port 8443 such that we do not need root privileges or extra capabilities for this server.
		// The Service object will take care of mapping this port to the HTTPS port 443.
		Addr:    fmt.Sprintf(":%v", port),
		Handler: mux,
	}
	err = server.ListenAndServeTLS(certFile, keyFile)

	if err != nil {
		panic(fmt.Sprintf("error unable to seart the server: %v", err))
	}
}
