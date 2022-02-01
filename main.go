package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	ctrlruntimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	ctx := context.Background()

	kubeconfig := os.Getenv("KUBECONFIG")
	verboseLog := false

	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "kubeconfig file to use")
	flag.BoolVar(&verboseLog, "verbose", verboseLog, "enable more verbose logging")
	flag.Parse()

	// setup logging
	var log = logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC1123,
	})

	if verboseLog {
		log.SetLevel(logrus.DebugLevel)
	}

	if flag.NArg() == 0 {
		log.Fatal("No CRD names provided.")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create kube client: %v", err)
	}

	client, err := ctrlruntimeclient.New(config, ctrlruntimeclient.Options{})
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	if err := apiextensionsv1.AddToScheme(client.Scheme()); err != nil {
		log.Fatalf("Failed to add apiextensions/v1 to scheme: %v", err)
	}

	success := false

	for _, crdName := range flag.Args() {
		crdName := strings.ToLower(crdName)
		crdLog := log.WithField("crd", crdName)

		if err := nukeCRD(ctx, crdLog, client, crdName); err != nil {
			crdLog.Errorf("Failed to nuke: %v", err)
			success = true
		}
	}

	if success {
		log.Info("Everything nuked successfully.")
	} else {
		os.Exit(1)
	}
}

func nukeCRD(ctx context.Context, log logrus.FieldLogger, client ctrlruntimeclient.Client, crdName string) error {
	log.Info("Nuking…")

	// fetch the CRD
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := client.Get(ctx, types.NamespacedName{Name: crdName}, crd); err != nil {
		if kerrors.IsNotFound(err) {
			log.Debug("CRD does not exist.")
			return nil
		}

		return fmt.Errorf("failed to retrieve CRD: %w", err)
	}

	// delete it, this will get rid of all CRs with no finalizers, i.e. less work for us to do
	if err := client.Delete(ctx, crd); err != nil {
		return fmt.Errorf("failed to delete CRD resource: %w", err)
	}

	// remove stuck resources
	if err := removeResources(ctx, log, client, crd); err != nil {
		return err
	}

	// check if the CRD is gone
	time.Sleep(3 * time.Second)

	crd = &apiextensionsv1.CustomResourceDefinition{}
	err := client.Get(ctx, types.NamespacedName{Name: crdName}, crd)
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("failed to check final CRD existence: %w", err)
	}
	if err == nil {
		log.Warn("CRD still exists, some resources might be blocked by owner references to them.")
	}

	return nil
}

func removeResources(ctx context.Context, log logrus.FieldLogger, client ctrlruntimeclient.Client, crd *apiextensionsv1.CustomResourceDefinition) error {
	if crd.Spec.Scope == apiextensionsv1.NamespaceScoped {
		nsList := &corev1.NamespaceList{}
		if err := client.List(ctx, nsList); err != nil {
			return fmt.Errorf("failed to list namespaces: %w", err)
		}

		for _, namespace := range nsList.Items {
			opt := &ctrlruntimeclient.ListOptions{
				Namespace: namespace.Name,
			}

			if err := removeResourcesWithOpts(ctx, log, client, crd, opt); err != nil {
				return err
			}
		}
	}

	return removeResourcesWithOpts(ctx, log, client, crd)
}

func removeResourcesWithOpts(ctx context.Context, log logrus.FieldLogger, client ctrlruntimeclient.Client, crd *apiextensionsv1.CustomResourceDefinition, opts ...ctrlruntimeclient.ListOption) error {
	apiVersion, err := getAPIVersion(crd)
	if err != nil {
		return err
	}

	objectList := &unstructured.UnstructuredList{}
	objectList.SetAPIVersion(apiVersion)
	objectList.SetKind(crd.Spec.Names.Kind)

	if err := client.List(ctx, objectList, opts...); err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	for _, obj := range objectList.Items {
		// this should not happen, unless an ownerRef with blockOwnerDeletion is in place
		if len(obj.GetFinalizers()) == 0 {
			continue
		}

		objIdent := obj.GetName()
		if ns := obj.GetNamespace(); len(ns) > 0 {
			objIdent = fmt.Sprintf("%s/%s", ns, objIdent)
		}

		log.WithField("resource", objIdent).Debug("Nuking…")

		oldObj := obj.DeepCopy()
		obj.SetFinalizers(nil)
		if err := client.Patch(ctx, &obj, ctrlruntimeclient.MergeFrom(oldObj)); err != nil {
			return fmt.Errorf("failed to delete %s: %w", objIdent, err)
		}
	}

	return nil
}

func getAPIVersion(crd *apiextensionsv1.CustomResourceDefinition) (string, error) {
	for _, version := range crd.Spec.Versions {
		if version.Served {
			return fmt.Sprintf("%s/%s", crd.Spec.Group, version.Name), nil
		}
	}

	return "", fmt.Errorf("CRD has no version marked as `served`")
}
