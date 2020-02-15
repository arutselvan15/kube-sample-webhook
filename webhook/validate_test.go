package webhook

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/arutselvan15/estore-product-kube-client/pkg/apis/estore/v1"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
)

func Test_validateBrand(t *testing.T) {
	type args struct {
		brand string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success valid brand", args: args{brand: "iphone"}, wantErr: false,
		}, {
			name: "failure invalid brand", args: args{brand: "@phone"}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateBrand(tt.args.brand); (err != nil) != tt.wantErr {
				t.Errorf("validateBrand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateName(t *testing.T) {
	type args struct {
		name string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success valid name", args: args{name: "iphone"}, wantErr: false,
		}, {
			name: "failure invalid name", args: args{name: "kube-@phone"}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateName(tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("validateName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateProduct(t *testing.T) {
	type args struct {
		operation string
		pdt       v1.Product
	}

	pdtErr := v1.Product{
		ObjectMeta: metav1.ObjectMeta{Name: "kube-iphone"},
		Spec:       v1.ProductSpec{Brand: "@iphone"},
	}

	pdtOk := v1.Product{
		ObjectMeta: metav1.ObjectMeta{Name: "iphone"},
		Spec:       v1.ProductSpec{Brand: "iphone"},
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success valid pdt", args: args{operation: cfg.Create, pdt: pdtOk}, wantErr: false,
		}, {
			name: "failure invalid pdt", args: args{operation: cfg.Update, pdt: pdtErr}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateProduct(tt.args.operation, tt.args.pdt); (err != nil) != tt.wantErr {
				t.Errorf("validateProduct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
