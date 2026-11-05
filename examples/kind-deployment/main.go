package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kubehealth-project/kubehealth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

var deploymentGVR = schema.GroupVersionResource{
	Group: "apps", Version: "v1", Resource: "deployments",
}

func main() {
	namespace := flag.String("namespace", "default", "namespace containing the Deployment")
	name := flag.String("name", "kubehealth-demo", "Deployment name")
	flag.Parse()

	if err := run(context.Background(), *namespace, *name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, namespace, name string) error {
	configOverrides := &clientcmd.ConfigOverrides{}
	if kubeContext := os.Getenv("KUBE_CONTEXT"); kubeContext != "" {
		configOverrides.CurrentContext = kubeContext
	}
	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(), configOverrides,
	)
	config, err := kubeconfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("load Kubernetes configuration: %w", err)
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create Kubernetes client: %w", err)
	}

	deployment, err := client.Resource(deploymentGVR).Namespace(namespace).Get(
		ctx, name, metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("get Deployment %s/%s: %w", namespace, name, err)
	}
	assessment, err := kubehealth.Assess(deployment)
	if err != nil {
		return fmt.Errorf("assess Deployment %s/%s: %w", namespace, name, err)
	}

	return printAssessment(deployment, assessment)
}

func printAssessment(deployment *unstructured.Unstructured, assessment kubehealth.Assessment) error {
	output, err := json.MarshalIndent(assessment, "", "  ")
	if err != nil {
		return fmt.Errorf("encode assessment: %w", err)
	}
	fmt.Printf("KubeHealth assessment for Deployment %s/%s:\n%s\n",
		deployment.GetNamespace(), deployment.GetName(), output)
	return nil
}
