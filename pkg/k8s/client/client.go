package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubemq-io/kubemqctl/pkg/k8s/client/v1beta1"
	"github.com/kubemq-io/kubemqctl/pkg/k8s/types"

	"github.com/kubemq-io/kubemqctl/pkg/utils"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	apiextension "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type Client struct {
	ClientSet          *kubernetes.Clientset
	ClientConfig       clientcmd.ClientConfig
	ClientApiExtension *apiextension.Clientset
	ClientV1Beta1      *v1beta1.V1Beta1Client
}

func NewClient(kubeConfigPath string) (*Client, error) {
	var kubeconfig string
	if kubeConfigPath != "" {
		kubeconfig = kubeConfigPath
	} else {
		if home := homeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		} else {
			return nil, errors.New("no kubeconfig available")
		}
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}})
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubernetes config file: %s", err)
	}
	// create the clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	clientExtension, err := apiextension.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	err = types.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}
	clientV1Beta1, err := v1beta1.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	c := &Client{
		ClientSet:          clientset,
		ClientConfig:       clientConfig,
		ClientApiExtension: clientExtension,
		ClientV1Beta1:      clientV1Beta1,
	}
	kubeCfg, _ := c.ClientConfig.ConfigAccess().GetStartingConfig()
	utils.Printlnf("Current Kubernetes cluster context connection: %s", kubeCfg.CurrentContext)
	return c, nil
}

func (c *Client) GetConfigClusters() (map[string]*clientcmdapi.Cluster, error) {
	config, err := c.ClientConfig.ConfigAccess().GetStartingConfig()
	if err != nil {
		return nil, err
	}
	return config.Clusters, nil
}

func (c *Client) GetConfigContext() (map[string]*clientcmdapi.Context, string, error) {
	config, err := c.ClientConfig.ConfigAccess().GetStartingConfig()
	if err != nil {
		return nil, "", err
	}
	return config.Contexts, config.CurrentContext, nil
}

func (c *Client) GetCurrentContext() (string, error) {
	config, err := c.ClientConfig.ConfigAccess().GetStartingConfig()
	if err != nil {
		return "", err
	}
	return config.CurrentContext, nil
}

func (c *Client) SwitchContext(contextName string) error {
	config, err := c.ClientConfig.ConfigAccess().GetStartingConfig()
	if err != nil {
		return err
	}
	config.CurrentContext = contextName
	err = clientcmd.ModifyConfig(c.ClientConfig.ConfigAccess(), *config, true)
	return err
}

func (c *Client) GetPods(ns string, name string) (map[string]apiv1.Pod, error) {
	pods, err := c.ClientSet.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	list := map[string]apiv1.Pod{}
	items := pods.Items
	for i := 0; i < len(items); i++ {
		item := items[i]
		if strings.Contains(item.Name, name+"-") {
			list[fmt.Sprintf("%s/%s", item.Namespace, item.Name)] = item
		}
	}
	return list, err
}

func (c *Client) GetServices(ns string, name string) (map[string]apiv1.Service, error) {
	pods, err := c.ClientSet.CoreV1().Services(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	list := map[string]apiv1.Service{}
	items := pods.Items
	for i := 0; i < len(items); i++ {
		item := items[i]
		if item.Name == name {
			list[fmt.Sprintf("%s/%s", item.Namespace, item.Name)] = item
		}
	}
	return list, err
}

func (c *Client) ForwardPorts(ns string, name string, ports []string, stopChan chan struct{}, outCh chan string, errOutCh chan string) error {
	restConfig, err := c.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}
	roundTripper, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", ns, name)
	hostIP := strings.Replace(restConfig.Host, "https://", "", -1) //nolint
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)
	readyChan := make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return err
	}

	go func() {
		for range readyChan { // Kubernetes will close this channel when it has something to tell us.
		}
		if len(errOut.String()) != 0 {
			errOutCh <- errOut.String()
			close(stopChan)
		} else if len(out.String()) != 0 {
			outCh <- out.String()
		}
	}()

	go func() {
		if err = forwarder.ForwardPorts(); err != nil { // Locks until stopChan is closed.
			errOutCh <- err.Error()
		}
	}()

	return nil
}

func (c *Client) ForwardPortsToService(ns string, name string, ports []string, stopChan chan struct{}, outCh chan string, errOutCh chan string) error {
	restConfig, err := c.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}
	roundTripper, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v1/namespaces/%s/service/%s/portforward", ns, name)
	hostIP := strings.Replace(restConfig.Host, "https://", "", -1) //nolint
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)
	readyChan := make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, out, errOut)
	if err != nil {
		return err
	}

	go func() {
		for range readyChan { // Kubernetes will close this channel when it has something to tell us.
		}
		if len(errOut.String()) != 0 {
			errOutCh <- errOut.String()
			close(stopChan)
		} else if len(out.String()) != 0 {
			outCh <- out.String()
		}
	}()

	go func() {
		if err = forwarder.ForwardPorts(); err != nil { // Locks until stopChan is closed.
			errOutCh <- err.Error()
		}
	}()

	return nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
