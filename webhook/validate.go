// Package webhook webhook
package webhook

import (
	"fmt"
	"regexp"
	"strings"

	pdtv1 "github.com/arutselvan15/estore-product-kube-client/pkg/apis/estore/v1"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
)

func validateProduct(operation string, pdt pdtv1.Product) error {
	var errors []string

	switch strings.ToLower(operation) {
	case strings.ToLower(cfg.Create), strings.ToLower(cfg.Update):
		if err := validateName(pdt.Name); err != nil {
			errors = append(errors, err.Error())
		}

		if err := validateBrand(pdt.Spec.Brand); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if errors != nil {
		return fmt.Errorf(strings.Join(errors, ". "))
	}

	return nil
}

func validateName(name string) error {
	if strings.HasPrefix(name, "kube-") {
		return fmt.Errorf("metadata.name %s with prefix kube- is not allowed", name)
	}

	return nil
}

func validateBrand(brand string) error {
	// match alphabets and - only
	ok, err := regexp.MatchString("^([a-zA-Z-]+$)", brand)
	if err != nil {
		return fmt.Errorf("unable to check spec.brand name pattern. %s", err.Error())
	}

	if !ok {
		return fmt.Errorf("spec.brand %s is not valid", brand)
	}

	return nil
}
