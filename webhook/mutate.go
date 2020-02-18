package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	cv "github.com/arutselvan15/estore-common/validate"
	pdtv1 "github.com/arutselvan15/estore-product-kube-client/pkg/apis/estore/v1"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
)

// MutateProduct mutate product
func MutateProduct(pdt pdtv1.Product, operation, user string) ([]byte, error) {
	var (
		addAnnotations       = map[string]string{}
		addLabels            = map[string]string{}
		availableAnnotations = pdt.GetAnnotations()
		availableLabels      = pdt.GetLabels()
	)

	if availableAnnotations == nil {
		availableAnnotations = map[string]string{}
	}

	if availableLabels == nil {
		availableLabels = map[string]string{}
	}

	if user == "" {
		return nil, fmt.Errorf("user not found in request")
	}

	if strings.EqualFold(operation, cfg.Delete) {
		return nil, nil
	}

	if strings.EqualFold(availableAnnotations[pdtv1.ProductAnnotationWebhookStatusKey], cfg.Mutated) {
		return nil, nil
	}

	// add annotation to mark the resource as mutated
	addAnnotations[pdtv1.ProductAnnotationWebhookStatusKey] = cfg.Mutated
	patch := cv.CreatePatchAnnotations(availableAnnotations, addAnnotations)
	patch = append(patch, cv.CreatePatchLabels(availableLabels, addLabels)...)

	return json.Marshal(patch)
}
