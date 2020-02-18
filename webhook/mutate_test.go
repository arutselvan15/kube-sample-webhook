package webhook

import (
	"testing"

	pdtv1 "github.com/arutselvan15/estore-product-kube-client/pkg/apis/estore/v1"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
)

func TestMutateProduct(t *testing.T) {
	type args struct {
		operation string
		user      string
		pdt       pdtv1.Product
	}

	pdt := createProduct("sample-ns", "sample-prd", "apple")
	alreadyMutatedPdt := pdt.DeepCopy()
	alreadyMutatedPdt.Annotations[pdtv1.ProductAnnotationWebhookStatusKey] = cfg.Mutated

	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "success mutate pdt", args: args{operation: cfg.Create, pdt: *pdt, user: "system"}, want: true,
		},
		{
			name: "failure mutate pdt no user", args: args{operation: cfg.Create, pdt: *pdt, user: ""}, want: false, wantErr: true,
		},
		{
			name: "success no mutate pdt on delete", args: args{operation: cfg.Delete, pdt: *pdt, user: "system"}, want: false,
		},
		{
			name: "success no mutate pdt already mutation done", args: args{operation: cfg.Update, pdt: *alreadyMutatedPdt, user: "system"}, want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MutateProduct(tt.args.pdt, tt.args.operation, tt.args.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("MutateProduct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got != nil) != tt.want {
				t.Errorf("MutateProduct() got = %v, want %v", got, tt.want)
			}
		})
	}
}
