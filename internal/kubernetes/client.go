package kubernetes

import (
	"os"

	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	ClientSet      *kubernetes.Clientset
	RestConfig     *rest.Config
	Connected      bool
	ClusterVersion string
	Options        *ClientOptions
}

type ClientOptions struct {
	Namespace      string
	KubeconfigPath string
}

func NewClient(options *ClientOptions) (*Client, error) {

	if options.KubeconfigPath == "" {
		log.Trace().Msg("No KUBECONFIG given. Testing default locations")
		// check if file exists
		_, err := os.Stat(clientcmd.RecommendedHomeFile)
		if os.IsNotExist(err) {
			log.Trace().Msg("No KUBECONFIG found in default locations. Attempting in-cluster config")
		} else if err == nil {
			log.Trace().Msgf("Found KUBECONFIG in default location: %s", clientcmd.RecommendedHomeFile)
			options.KubeconfigPath = clientcmd.RecommendedHomeFile
		}
	}

	var (
		config *rest.Config
		err    error
	)

	if options.KubeconfigPath != "" {
		log.Trace().Msgf("Building Kubernetes client config from KUBECONFIG: %s", options.KubeconfigPath)
		config, err = clientcmd.BuildConfigFromFlags("", options.KubeconfigPath)
		if err != nil {
			log.Warn().Msgf("Failed to load cluster config from %s", options.KubeconfigPath)
			return nil, err
		}
	} else {
		log.Trace().Msg("Attempting to build in-cluster Kubernetes client config")
		config, err = rest.InClusterConfig()
		if err != nil {
			log.Warn().Msgf("Failed to load in-cluster config: %v", err)
			log.Trace().Msg("Falling back to default kubeconfig loading rules")
			config, err = clientcmd.BuildConfigFromFlags("", "")
			if err != nil {
				log.Warn().Msg("No default cluster config available")
				return nil, err
			}
		}
	}

	log.Trace().Msg("Creating Kubernetes clientset")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	clusterClient := &Client{
		ClientSet:  clientset,
		RestConfig: config,
		Connected:  false,
		Options:    options,
	}
	clusterClient.TestConnection()

	return clusterClient, nil
}

func (c *Client) TestConnection() (bool, error) {
	log.Trace().Msg("Testing Kubernetes cluster connection")
	serverVersion, err := c.ClientSet.Discovery().ServerVersion()
	if err != nil {
		log.Warn().Msgf("Failed to connect to Kubernetes cluster: %v", err)
		c.Connected = false
		return false, err
	}
	log.Debug().Msgf("Connected to Kubernetes cluster: %v", serverVersion)
	c.Connected = true
	c.ClusterVersion = serverVersion.String()
	return true, nil
}
