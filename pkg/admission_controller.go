package pkg

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"fmt"

	"k8s.io/klog"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	v1 "k8s.io/kubernetes/pkg/apis/core/v1"
)

var (
	logcleanKey = "logclean.daocloud.io/name:"
	logcleanValue = "logclean-job"
)

// This is used only for specific objects
var (
	priorityClassReplace = patchOperation{
		Op: "replace",
		Path: "/spec/priorityClassName",
		Value: "high-priority",
	}
	schedulerPathchType = v1beta1.PatchTypeJSONPatch
	runtimeScheme = runtime.NewScheme()
	deserializer = serializer.NewCodecFactory(runtimeScheme).UniversalDeserializer()
)

// Web hook server parameters
type WhSvrParameters struct {
	Port int // webhook server port
	CertFile string // path to the x509 certificates for https
	KeyFile string // path to the x509 private key matching `CertFile`
	SidecarCfgFile string // path to sidecar injector configuration file
}

type patchOperation struct {
	Op string `json:"op"`
	Path string `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func init() {
	_ = corev1.AddToScheme(runtimeScheme)
	_ = admissionregistrationv1beta1.AddToScheme(runtimeScheme)
	_ = v1.AddToScheme(runtimeScheme)
}

type WebhookServer struct {
	Server *http.Server
}

func requiredMutation(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.ObjectMeta.Labels != nil && pod.ObjectMeta.Labels[logcleanKey] == logcleanValue {
		return true
	}
	return false
}

// main mutation process
func (whsvr *WebhookServer) mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request
	if req.Kind.Kind == "Pod" {
		var pod corev1.Pod
		if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
			klog.Errorf("Could not unmarshal raw object: %v", err)
			return &v1beta1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		if requiredMutation(&pod) {
			patchBytes,_ := json.Marshal([]patchOperation{priorityClassReplace})
			klog.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
			return &v1beta1.AdmissionResponse{
				Allowed: true,
				Patch: patchBytes,
				PatchType: &schedulerPathchType,
			}
		} else {
			return &v1beta1.AdmissionResponse{
				Allowed: true,
			}
		}
	} else {
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
}

// Serve method for web hook server
func (whsvr *WebhookServer) Serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		klog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *v1beta1.AdmissionResponse
	ar := v1beta1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		klog.Errorf("Can't decode body: %v", err)
		admissionResponse = &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		klog.Infof("Request path: %v", r.URL.Path)
		if r.URL.Path == "/mutate" {
			admissionResponse = whsvr.mutate(&ar)
		}
	}

	admissionReview := v1beta1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}

	resp, err := json.Marshal(admissionReview)
	if err != nil {
		klog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("clould not encode response: %v", err), http.StatusInternalServerError)
	}
	klog.Info("Ready to write response...")
	if _, err := w.Write(resp); err != nil {
		klog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
