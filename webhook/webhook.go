// Package webhook webhook
package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	cc "github.com/arutselvan15/estore-common/clients"
	ccfg "github.com/arutselvan15/estore-common/config"
	cv "github.com/arutselvan15/estore-common/validate"
	pdtv1 "github.com/arutselvan15/estore-product-kube-client/pkg/apis/estore/v1"
	lc "github.com/arutselvan15/go-utils/logconstants"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
	cLog "github.com/arutselvan15/estore-product-kube-webhook/log"
)

var (
	pdt               pdtv1.Product
	admissionResponse *v1beta1.AdmissionResponse

	log           = cLog.GetLogger()
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

// Server server
type Server struct {
	Clients cc.EstoreClientInterface
}

// Serve serve
func (s Server) Serve(rWriter http.ResponseWriter, req *http.Request) {
	admissionReviewReceived, err := decodeAdmissionReview(req.Body)

	if err != nil {
		log.Errorf(err.Error())
		http.Error(rWriter, "empty body", http.StatusBadRequest)

		return
	}

	arRequest := admissionReviewReceived.Request

	// during delete the object will be empty but you will have old object
	objBytes := arRequest.Object.Raw
	if strings.EqualFold(string(arRequest.Operation), cfg.Delete) {
		objBytes = arRequest.OldObject.Raw
	}

	if err = json.Unmarshal(objBytes, &pdt); err != nil {
		log.SetStepState(lc.Error).Errorf("can't unmarshal product object: %s", err.Error())

		admissionResponse = &v1beta1.AdmissionResponse{Result: &metav1.Status{Message: err.Error()}}
	} else {
		log.SetObjectName(arRequest.Name).SetOperation(strings.ToLower(string(arRequest.Operation))).SetUser(
			arRequest.UserInfo.Username).Infof("admission review for namespace=%s, name=%s, user=%s, operation=%s",
			arRequest.Namespace, arRequest.Name, arRequest.UserInfo.Username, arRequest.Operation)

		log.LogAuditObject(pdt)

		if req.URL.Path == cfg.MutateURL {
			admissionResponse = s.mutate(string(arRequest.Operation), pdt)
		} else if req.URL.Path == cfg.ValidateURL {
			admissionResponse = s.validate(string(arRequest.Operation), pdt)
		}
	}

	admissionReviewOut := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReviewOut.Response = admissionResponse
		if admissionReviewReceived != nil && admissionReviewReceived.Request != nil {
			admissionReviewOut.Response.UID = admissionReviewReceived.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReviewOut)
	if err != nil {
		log.SetStepState(lc.Error).Errorf("can't encode response: %v", err)
		http.Error(rWriter, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := rWriter.Write(resp); err != nil {
		log.SetStepState(lc.Error).Errorf("can't write response: %v", err)
		http.Error(rWriter, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	} else {
		if admissionResponse != nil && admissionResponse.Allowed {
			log.SetStepState(lc.Complete).Info("admission review completed successfully")
		} else {
			log.SetStepState(lc.Error).Info("admission review failed")
		}
	}
}

func (s Server) mutate(operation string, pdt pdtv1.Product) *v1beta1.AdmissionResponse {
	log.SetStep(lc.Mutate).SetStepState(lc.Start).Infof("mutate namespace=%s, name=%s", pdt.Namespace, pdt.Name)

	response := &v1beta1.AdmissionResponse{
		Allowed: false,
		Result:  &metav1.Status{},
	}

	required, msg := cv.AdmissionRequired(ccfg.WhitelistNamespaces, pdtv1.ProductAnnotationWebhookMutateKey, &pdt.ObjectMeta)

	if !required {
		log.SetStepState(lc.Skip).Info(msg)

		response.Allowed = true
	} else {
		patchBytes, err := MutateProduct(operation, pdt)

		if err != nil {
			log.SetStepState(lc.Error).Error(err.Error())

			response.Result.Message = err.Error()
		} else {
			if patchBytes != nil {
				log.Infof("patch resource : %s", string(patchBytes))

				response.Patch = patchBytes
				response.PatchType = func() *v1beta1.PatchType {
					pt := v1beta1.PatchTypeJSONPatch
					return &pt
				}()
			} else {
				log.SetStepState(lc.Skip).Info("mutation not applicable")
			}

			response.Allowed = true
		}
	}

	return response
}

func (s Server) validate(operation string, pdt pdtv1.Product) *v1beta1.AdmissionResponse {
	log.SetStep(lc.Validate).SetStepState(lc.Start).Infof("validate namespace=%s, name=%s", pdt.Namespace, pdt.Name)

	response := &v1beta1.AdmissionResponse{
		Allowed: false,
		Result:  &metav1.Status{},
	}

	required, msg := cv.AdmissionRequired(ccfg.WhitelistNamespaces, pdtv1.ProductAnnotationWebhookValidateKey, &pdt.ObjectMeta)

	if !required {
		log.SetStepState(lc.Skip).Info(msg)

		response.Allowed = true
	} else {
		err := validateProduct(operation, pdt)

		if err != nil {
			log.SetStepState(lc.Error).Error(err.Error())
			response.Result.Message = err.Error()
		} else {
			// log audit object
			log.SetObjectState(lc.Received).LogAuditObject(pdt)
			response.Allowed = true
		}
	}

	return response
}

func decodeAdmissionReview(httpBody io.Reader) (*v1beta1.AdmissionReview, error) {
	var body []byte

	if httpBody != nil {
		if data, err := ioutil.ReadAll(httpBody); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("request body is empty")
	}

	// decode request
	admissionReviewReceived := &v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, admissionReviewReceived); err != nil {
		// error decoding the request
		return nil, fmt.Errorf("can't decode body: %s", err.Error())
	}

	return admissionReviewReceived, nil
}
