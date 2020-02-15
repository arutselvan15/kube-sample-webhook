package main

import (
	"flag"
	"net/http"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cc "github.com/arutselvan15/estore-common/clients"
	gc "github.com/arutselvan15/estore-common/config"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
	gLog "github.com/arutselvan15/estore-product-kube-webhook/log"
	"github.com/arutselvan15/estore-product-kube-webhook/webhook"
)

func main() {
	var (
		port, certFile, keyFile string
		config                  *rest.Config
		err                     error
	)

	flag.StringVar(&port, "port", "8000", "Webhook server port.")
	flag.StringVar(&certFile, "tlsCertFile", "/etc/webhook/certs/estore-product-kube-webhook.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&keyFile, "tlsKeyFile", "/etc/webhook/certs/estore-product-kube-webhook.key", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()

	log := gLog.GetLogger()

	// kube config defined in env
	if gc.KubeConfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", gc.KubeConfigPath)
		if err != nil {
			log.Fatalf("error creating config using kube config path: %v", err)
		}
	} else {
		// default get current cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Fatalf("error creating config using cluster config: %v", err)
		}
	}

	// create client for config
	estoreClients, err := cc.NewEstoreClientForConfig(config)
	if err != nil {
		log.Fatalf("error creating clients: %v", err)
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
		Addr:    port,
		Handler: mux,
	}
	log.Fatal(server.ListenAndServeTLS(certFile, keyFile))
}