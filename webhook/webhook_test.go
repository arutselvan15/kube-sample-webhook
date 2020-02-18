package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/admission/v1beta1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/arutselvan15/estore-common/clients"
	ccFake "github.com/arutselvan15/estore-common/clients/fake"
	cc "github.com/arutselvan15/estore-common/config"
	pdtv1 "github.com/arutselvan15/estore-product-kube-client/pkg/apis/estore/v1"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
)

func createProduct(namespace, name, brand string) *pdtv1.Product {
	pdt := &pdtv1.Product{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{},
			Labels:      map[string]string{},
		},
		Spec: pdtv1.ProductSpec{
			Brand: brand,
		},
	}

	return pdt
}

func genUUID() types.UID {
	uid, _ := uuid.NewUUID()
	return types.UID(uid.String())
}

func createAdmissionReview(pdt *pdtv1.Product, username string, operation v1beta1.Operation) *v1beta1.AdmissionReview {
	pdtObj, _ := json.Marshal(&pdt)

	return &v1beta1.AdmissionReview{
		Request: &v1beta1.AdmissionRequest{
			Kind:      metav1.GroupVersionKind{},
			Namespace: pdt.Namespace,
			Name:      pdt.Name,
			UID:       genUUID(),
			Operation: operation,
			UserInfo:  authenticationv1.UserInfo{Username: username},
			Object: runtime.RawExtension{
				Raw: pdtObj,
			},
		},
	}
}

func Test_decodeAdmissionReview(t *testing.T) {
	type args struct {
		httpBody io.Reader
	}

	tests := []struct {
		name    string
		args    args
		want    *v1beta1.AdmissionReview
		wantErr bool
	}{
		{
			name: "failure empty body", args: args{httpBody: nil}, want: nil, wantErr: true,
		},
		{
			name: "failure invalid admission review request", args: args{httpBody: strings.NewReader("invalidAdmissionReview")},
			want: nil, wantErr: true,
		},
		{
			name: "success decode admission request",
			args: args{httpBody: strings.NewReader(`{ "apiVersion": "dummy version", "kind": "dummy kind" }`)},
			want: &v1beta1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{
					Kind:       "dummy kind",
					APIVersion: "dummy version",
				},
				Request:  nil,
				Response: nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeAdmissionReview(tt.args.httpBody)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeAdmissionReview() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decodeAdmissionReview() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServer_Serve(t *testing.T) {
	_ = cc.LoadFixture(cc.FixtureDir)

	type fields struct {
		Clients clients.EstoreClientInterface
	}

	pdt := createProduct("sample-ns", "sample-prd", "apple")
	ar := createAdmissionReview(pdt, "testuser", cfg.Create)

	arSystemUser, arBlacklistUser, arBlacklistNs := ar.DeepCopy(), ar.DeepCopy(), ar.DeepCopy()
	arSystemUser.Request.UserInfo.Username = strings.Split(viper.GetString("app.system.users"), ",")[0]
	arBlacklistUser.Request.UserInfo.Username = strings.Split(viper.GetString("app.blacklist.users"), ",")[0]
	arBlacklistNs.Request.Namespace = strings.Split(viper.GetString("app.blacklist.namespaces"), ",")[0]

	type args struct {
		admissionReview *v1beta1.AdmissionReview
		httpResp        int
		allowed         bool
	}

	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{name: "failure empty body", args: args{admissionReview: nil, httpResp: http.StatusOK, allowed: false}, fields: fields{Clients: ccFake.NewEstoreFakeClientForConfig(nil, nil)}},
		{name: "invalid http  body", args: args{admissionReview: &v1beta1.AdmissionReview{}, httpResp: http.StatusOK, allowed: false}, fields: fields{Clients: ccFake.NewEstoreFakeClientForConfig(nil, nil)}},
		{name: "success valid admission request", args: args{admissionReview: ar, httpResp: http.StatusOK, allowed: true}, fields: fields{Clients: ccFake.NewEstoreFakeClientForConfig(nil, nil)}},
		{name: "success system user", args: args{admissionReview: arSystemUser, httpResp: http.StatusOK, allowed: true}, fields: fields{Clients: ccFake.NewEstoreFakeClientForConfig(nil, nil)}},
		{name: "failure black list user", args: args{admissionReview: arBlacklistUser, httpResp: http.StatusOK, allowed: false}, fields: fields{Clients: ccFake.NewEstoreFakeClientForConfig(nil, nil)}},
		{name: "failure black list namespace", args: args{admissionReview: arBlacklistNs, httpResp: http.StatusOK, allowed: false}, fields: fields{Clients: ccFake.NewEstoreFakeClientForConfig(nil, nil)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			body, _ := json.Marshal(tt.args.admissionReview)
			request, _ := http.NewRequest("POST", cfg.ValidateURL, bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")

			s := Server{Clients: tt.fields.Clients}

			s.Serve(recorder, request)
			if tt.args.httpResp == http.StatusOK {
				res, err := decodeAdmissionReview(recorder.Body)
				if err != nil {
					t.Errorf("serve() error = %v, want %v", err, tt.args.httpResp)
				}

				if res.Response.Allowed != tt.args.allowed {
					t.Errorf("serve() allowed = %v, want %v, msg: %v", res.Response.Allowed, tt.args.allowed, res.Response.Result.Message)
				}
			}

			assert.Equal(t, recorder.Code, tt.args.httpResp, "want = %d, got = %d", tt.args.httpResp, recorder.Code)
		})
	}
}

func TestServer_validate(t *testing.T) {
	type fields struct {
		Clients clients.EstoreClientInterface
	}

	type args struct {
		operation string
		user      string
		pdt       pdtv1.Product
	}

	pdt := createProduct("sample-ns", "sample-prd", "apple")
	invalidName := createProduct("sample-ns", "kube-sample-prd", "apple")
	invalidBrand := createProduct("sample-ns", "sample-prd", "1apple")
	fClient := ccFake.NewEstoreFakeClientForConfig(nil, nil)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success validate pdt", fields: fields{Clients: fClient}, args: args{operation: cfg.Create, pdt: *pdt, user: "system"}, wantErr: false,
		},
		{
			name: "failure validate pdt invalid name", fields: fields{Clients: fClient}, args: args{operation: cfg.Update, pdt: *invalidName, user: "system"}, wantErr: true,
		},
		{
			name: "failure validate pdt invalid brand", fields: fields{Clients: fClient}, args: args{operation: cfg.Create, pdt: *invalidBrand, user: "system"}, wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Server{
				Clients: tt.fields.Clients,
			}
			err := s.validate(tt.args.pdt, tt.args.operation, tt.args.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestServer_mutate(t *testing.T) {
	type fields struct {
		Clients clients.EstoreClientInterface
	}

	type args struct {
		operation string
		user      string
		pdt       pdtv1.Product
	}

	pdt := createProduct("sample-ns", "sample-prd", "apple")
	alreadyMutatedPdt := pdt.DeepCopy()
	alreadyMutatedPdt.Annotations[pdtv1.ProductAnnotationWebhookStatusKey] = cfg.Mutated
	fClient := ccFake.NewEstoreFakeClientForConfig(nil, nil)

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "success mutate pdt", fields: fields{Clients: fClient}, args: args{operation: cfg.Create, pdt: *pdt, user: "system"}, want: true, wantErr: false,
		},
		{
			name: "success already mutate pdt", fields: fields{Clients: fClient}, args: args{operation: cfg.Update, pdt: *alreadyMutatedPdt, user: "system"}, want: false, wantErr: false,
		},
		{
			name: "success no mutate on delete", fields: fields{Clients: fClient}, args: args{operation: cfg.Delete, pdt: *pdt, user: "system"}, want: false, wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Server{
				Clients: tt.fields.Clients,
			}
			got, err := s.mutate(tt.args.pdt, tt.args.operation, tt.args.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("mutate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if (got != nil) != tt.want {
				t.Errorf("mutate() = %v, want nil %v", got, tt.want)
			}
		})
	}
}

func TestServer_handle(t *testing.T) {
	type fields struct {
		Clients clients.EstoreClientInterface
	}

	pdt := createProduct("sample-ns", "sample-prd", "apple")
	ar := createAdmissionReview(pdt, "testuser", cfg.Create)

	type args struct {
		reqPath string
		req     *v1beta1.AdmissionRequest
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "success mutate",
			args:   args{reqPath: cfg.MutateURL, req: ar.Request},
			fields: fields{Clients: ccFake.NewEstoreFakeClientForConfig(nil, nil)},
			want:   true,
		},
		{
			name:   "success validate",
			args:   args{reqPath: cfg.ValidateURL, req: ar.Request},
			fields: fields{Clients: ccFake.NewEstoreFakeClientForConfig(nil, nil)},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Server{
				Clients: tt.fields.Clients,
			}
			got := s.handle(tt.args.reqPath, tt.args.req)
			if got.Allowed != tt.want {
				t.Errorf("handle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_handleError(t *testing.T) {
	handleError(httptest.NewRecorder(), fmt.Errorf("test"), 400)
}
