package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml" // Required for yaml.Marshal

	applicationv1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	releasev1alpha1 "github.com/konflux-ci/release-service/api/v1alpha1"
)

const (
	defaultNamespace = "rh-acs-tenant"
)

func main() {
	// Define command-line flags
	environment := flag.String("environment", "", "ENVIRONMENT - allowed values: staging|prod")
	releaseNameSuffix := flag.String("release-name-suffix", "", "RELEASE_NAME_SUFFIX - for production, use something like acs-4-6-x-1; for staging acs-4-6-x-staging-1")
	operatorIndexCommit := flag.String("operator-index-commit", "", "OPERATOR_INDEX_COMMIT - default: currently checked out commit")
	flag.Parse()

	// Validate required command-line arguments
	if *environment == "" || *releaseNameSuffix == "" {
		fmt.Println("USAGE: go run main.go --environment <ENVIRONMENT> --release-name-suffix <RELEASE_NAME_SUFFIX> [--operator-index-commit <OPERATOR_INDEX_COMMIT>]")
		fmt.Println("")
		fmt.Println("ENVIRONMENT - allowed values: staging|prod")
		fmt.Println("RELEASE_NAME_SUFFIX - for production, use something like acs-4-6-x-1; for staging acs-4-6-x-staging-1")
		fmt.Println("OPERATOR_INDEX_COMMIT - default: currently checked out commit")
		fmt.Println("")
		fmt.Println("You must have your KUBECONFIG point to the Konflux cluster, see https://spaces.redhat.com/pages/viewpage.action?pageId=407312060#HowtoeverythingKonflux/RHTAPforRHACS-GettingocCLItoworkwithKonflux.")
		os.Exit(1)
	}

	// Validate ENVIRONMENT input
	if *environment != "staging" && *environment != "prod" {
		fmt.Printf("ERROR: ENVIRONMENT input must either be 'staging' or 'prod'. Got: %s\n", *environment)
		os.Exit(1)
	}

	// If operatorIndexCommit is not provided, get the current git commit SHA
	if *operatorIndexCommit == "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		output, err := cmd.Output()
		if err != nil {
			fmt.Printf("ERROR: Could not get current git HEAD: %v\n", err)
			os.Exit(1)
		}
		*operatorIndexCommit = strings.TrimSpace(string(output))
		fmt.Printf("Using current git HEAD as OPERATOR_INDEX_COMMIT: %s\n", *operatorIndexCommit)
	}

	// Build Kubernetes config from KUBECONFIG environment variable
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Create Kubernetes clients
	// We need a standard clientset for core resources (though not strictly used in this script)
	// and a dynamic client for custom resources like Snapshots and Releases.
	_, err = kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating Kubernetes clientset: %v\n", err)
		os.Exit(1)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating dynamic Kubernetes client: %v\n", err)
		os.Exit(1)
	}

	// Define the GroupVersionResource for the Snapshot CRD
	snapshotGVR := schema.GroupVersionResource{
		Group:    "appstudio.redhat.com",
		Version:  "v1alpha1",
		Resource: "snapshots",
	}

	// Validate snapshot existence by listing snapshots with the specific SHA label
	snapshotList, err := dynamicClient.Resource(snapshotGVR).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("pac.test.appstudio.openshift.io/sha=%s", *operatorIndexCommit),
	})
	if err != nil {
		fmt.Printf("Error listing snapshots for commit '%s': %v\n", *operatorIndexCommit, err)
		os.Exit(1)
	}

	if len(snapshotList.Items) == 0 {
		fmt.Printf("ERROR: Could not find any Snapshots for the commit '%s'. This must be a 40 character-long commit SHA. Default: currently checked out commit.\n", *operatorIndexCommit)
		os.Exit(1)
	}

	// Iterate through found snapshots and generate Release YAMLs
	for _, item := range snapshotList.Items {
		// Convert the unstructured object to our typed Snapshot struct
		var snapshot applicationv1alpha1.Snapshot
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &snapshot)
		if err != nil {
			fmt.Printf("Error converting unstructured to Snapshot: %v\n", err)
			continue // Skip to the next snapshot if conversion fails
		}

		// Construct the releasePlan name based on the environment
		releasePlan := strings.Replace(snapshot.Spec.Application, "acs-operator-index", fmt.Sprintf("acs-operator-index-%s", *environment), 1)

		// Create the Release object
		release := releasev1alpha1.Release{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "appstudio.redhat.com/v1alpha1", // Correct APIVersion for Release
				Kind:       "Release",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", snapshot.Spec.Application, *releaseNameSuffix),
				Namespace: defaultNamespace, // Namespace as defined in the original script
			},
			Spec: releasev1alpha1.ReleaseSpec{
				ReleasePlan: releasePlan,
				Snapshot:    snapshot.ObjectMeta.Name, // Use the name of the Snapshot as required by ReleaseSpec
			},
		}

		// Marshal the Release object to YAML format
		yamlBytes, err := yaml.Marshal(release)
		if err != nil {
			fmt.Printf("Error marshalling Release to YAML: %v\n", err)
			continue // Skip to the next snapshot if marshalling fails
		}

		// Print the YAML output, separated by "---" as in the original script
		fmt.Printf("---\n%s\n", string(yamlBytes))
	}
}
