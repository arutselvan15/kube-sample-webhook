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
	cv "github.com/arutselvan15/estore-common/validate"
	pdtv1 "github.com/arutselvan15/estore-product-kube-client/pkg/apis/estore/v1"
	lc "github.com/arutselvan15/go-utils/logconstants"

	cfg "github.com/arutselvan15/estore-product-kube-webhook/config"
	cLog "github.com/arutselvan15/estore-product-kube-webhook/log"
)

var (
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
func (s Server) Serve(httpWriter http.ResponseWriter, httpReq *http.Request) {
	var (
		admissionReviewResponse = v1beta1.AdmissionReview{}
		admissionResponse       = &v1beta1.AdmissionResponse{
			Allowed: false,
			Result:  &metav1.Status{},
		}
	)

	admissionReviewRequest, err := decodeAdmissionReview(httpReq.Body)
	if err != nil {
		handleError(httpWriter, fmt.Errorf("empty body.  %s", err.Error()), http.StatusBadRequest)

		return
	}

	req := admissionReviewRequest.Request

	if req != nil {
		if cv.CheckBlacklistUser(req.UserInfo.Username) {
			admissionResponse.Result.Message = fmt.Sprintf("user %s is black listed", req.UserInfo.Username)
		} else if cv.CheckBlacklistNamespace(req.Namespace) {
			admissionResponse.Result.Message = fmt.Sprintf("namedpace %s is black listed", req.Namespace)
		} else if cv.CheckSystemUser(req.UserInfo.Username) || cv.CheckSystemNamespace(req.Namespace) {
			admissionResponse.Allowed = true
		} else {
			admissionResponse = s.handle(httpReq.URL.Path, req)
		}
	} else {
		admissionResponse.Result.Message = fmt.Sprintf("request is empty")
	}

	if admissionResponse != nil {
		admissionReviewResponse.Response = admissionResponse
		if admissionReviewRequest != nil && admissionReviewRequest.Request != nil {
			admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		handleError(httpWriter, fmt.Errorf("can't encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := httpWriter.Write(resp); err != nil {
		handleError(httpWriter, fmt.Errorf("can't write response: %v", err), http.StatusInternalServerError)
	} else {
		if admissionResponse != nil && admissionResponse.Allowed {
			log.SetStepState(lc.Complete).Info("admission review completed successfully")
		} else {
			log.SetStepState(lc.Error).Info("admission review failed")
		}
	}
}

func (s Server) handle(reqPath string, req *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	var (
		pdt        pdtv1.Product
		oldPdt     pdtv1.Product
		patchBytes []byte
		err        error

		response = &v1beta1.AdmissionResponse{Allowed: false, Result: &metav1.Status{}}
	)

	newObjBytes := req.Object.Raw
	oldObjBytes := req.OldObject.Raw

	if strings.EqualFold(string(req.Operation), cfg.Delete) {
		newObjBytes = oldObjBytes
	}

	if err = json.Unmarshal(newObjBytes, &pdt); err != nil {
		response.Result.Message = fmt.Sprintf("can't unmarshal product object: %s", err.Error())
	} else {
		log.SetObjectName(req.Name).SetOperation(strings.ToLower(string(req.Operation))).SetUser(
			req.UserInfo.Username).Infof("admission review for namespace=%s, name=%s, user=%s, operation=%s",
			req.Namespace, req.Name, req.UserInfo.Username, req.Operation)

		if reqPath == cfg.MutateURL {
			patchBytes, err = s.mutate(pdt, string(req.Operation), req.UserInfo.Username)
			if err == nil {
				response.Patch = patchBytes
				response.PatchType = func() *v1beta1.PatchType {
					pt := v1beta1.PatchTypeJSONPatch
					return &pt
				}()
			}
		} else if reqPath == cfg.ValidateURL {
			if strings.EqualFold(string(req.Operation), cfg.Update) {
				if err = json.Unmarshal(oldObjBytes, &oldPdt); err != nil {
					log.LogAuditObject(pdt)
				} else {
					log.LogAuditObject(oldPdt, pdt)
				}
			} else {
				log.LogAuditObject(pdt)
			}

			err = s.validate(pdt, string(req.Operation), req.UserInfo.Username)
		} else {
			err = fmt.Errorf("invalid request path %s", reqPath)
		}

		if err != nil {
			log.SetStepState(lc.Error).Error(err.Error())
			response.Result.Message = err.Error()
		} else {
			response.Allowed = true
		}
	}

	return response
}

func (s Server) mutate(pdt pdtv1.Product, operation, user string) ([]byte, error) {
	var (
		patchBytes []byte
		err        error
	)

	log.SetStep(lc.Mutate).SetStepState(lc.Start).Infof(
		"========== mutate namespace=%s, name=%s, operation=%s ==========", pdt.Namespace, pdt.Name, operation)

	required, msg := cv.AdmissionRequired(pdtv1.ProductAnnotationWebhookMutateKey, &pdt.ObjectMeta)

	if !required {
		log.SetStepState(lc.Skip).Info(msg)
	} else {
		patchBytes, err = MutateProduct(pdt, operation, user)

		if err == nil {
			if patchBytes != nil {
				log.Debugf("patch resource : %s", string(patchBytes))
			} else {
				log.SetStepState(lc.Skip).Info("mutation not applicable")
			}
		}
	}

	return patchBytes, err
}

func (s Server) validate(pdt pdtv1.Product, operation, user string) error {
	log.SetStep(lc.Validate).SetStepState(lc.Start).Infof(
		"========== validate namespace=%s, name=%s, operation=%s ==========", pdt.Namespace, pdt.Name, operation)

	required, msg := cv.AdmissionRequired(pdtv1.ProductAnnotationWebhookValidateKey, &pdt.ObjectMeta)

	if !required {
		log.SetStepState(lc.Skip).Info(msg)
	} else {
		err := validateProduct(pdt, operation, user)

		if err != nil {
			return err
		}
		log.SetObjectState(lc.Received).LogAuditObject(pdt)
	}

	return nil
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

func handleError(rWriter http.ResponseWriter, err error, errCode int) {
	log.SetStepState(lc.Error).Errorf(err.Error())
	http.Error(rWriter, fmt.Sprintf("error occurred: %v", err), errCode)
}
