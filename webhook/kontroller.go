package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"log"
	"net/http"
	"os"

	"github.com/getsentry/sentry-go"
	admissionv1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	scheme = runtime.NewScheme()
	codecs = serializer.NewCodecFactory(scheme)
	logger = log.New(os.Stdout, "info: ", log.LstdFlags)
)

func admissionReviewFromRequest(r *http.Request, deserializer runtime.Decoder) (*admissionv1.AdmissionReview, error) {
	// Validate that the incoming content type is correct.
	if r.Header.Get("Content-Type") != "application/json" {
		sentry.CaptureMessage("expected application/json content-type")
		return nil, fmt.Errorf("expected application/json content-type")
	}

	// Get the body data, which will be the AdmissionReview
	// content for the request.
	var body []byte
	if r.Body != nil {
		requestData, err := ioutil.ReadAll(r.Body)
		if err != nil {
			sentry.CaptureException(err)
			return nil, err
		}
		body = requestData
	}

	// Decode the request body into
	admissionReviewRequest := &admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, admissionReviewRequest); err != nil {
		sentry.CaptureException(err)
		return nil, err
	}

	return admissionReviewRequest, nil
}

func ValidatePod(w http.ResponseWriter, r *http.Request) {

	logger.Printf("Validation controller was called")

	deserializer := codecs.UniversalDeserializer()

	// Parse the AdmissionReview from the http request.
	admissionReviewRequest, err := admissionReviewFromRequest(r, deserializer)
	if err != nil {
		sentry.CaptureException(err)
		msg := fmt.Sprintf("error getting admission review from request: %v", err)
		logger.Printf(msg)
		w.WriteHeader(400)
		w.Write([]byte(msg))
		return
	}

	admissionResponse := &admissionv1.AdmissionResponse{}
	admissionResponse.Allowed = true
	logger.Printf(string(admissionReviewRequest.Request.Operation))
	//NS := admissionReviewRequest.Request.Namespace
	//logger.Printf(NS)
	if string(admissionReviewRequest.Request.Operation) == "DELETE" {
		admissionResponse.Allowed = false
		admissionResponse.Result = &metav1.Status{
			Message: "No Delete operation",
		}
	}
	/*
		// note : Need to add extra check because Affinity is struct and *ref to podspec
		if (NS == "sre") || (NS == "kube-system") || (NS == "default") {
			admissionResponse.Allowed = false
			admissionResponse.Result = &metav1.Status{
				Message: "No Delete allowed operation on critical Namespace",
			}
		}

			else {
				podList, err := podCounter(NS)
				if err != nil {
					panic(err.Error())
				}
				svcList, err := serviceCounter(NS)
				if err != nil {
					panic(err.Error())
				}
				pvcList, err := pvcCounter(NS)
				if err != nil {
					panic(err.Error())
				}
				if (podList > 0) || (svcList > 0) || (pvcList > 0) {
					admissionResponse.Allowed = false
					admissionResponse.Result = &metav1.Status{
						Message: "The Requested Namespace Contains Kubernetes objects, So the Delete request is rejected, Contact Admin",
					}
				}
			}
	*/
	var admissionReviewResponse admissionv1.AdmissionReview
	admissionReviewResponse.Response = admissionResponse
	admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID

	resp, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		sentry.CaptureException(err)
		msg := fmt.Sprintf("error marshalling response json: %v", err)
		logger.Printf(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		valkontrollerCounter.WithLabelValues("5xx", "fail").Inc()
		return
	}
	valkontrollerCounter.WithLabelValues("200", "success").Inc()
	rejectionCounter.WithLabelValues(admissionReviewRequest.Request.Namespace, admissionReviewRequest.Request.Name).Inc()
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}
func inclusterConfig() (dynamic.Interface, error) {

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		fmt.Printf("error creating dynamic client: %v\n", err)
		os.Exit(1)
	}
	return dynamicClient, nil

}

func podCounter(namespace string) (int, error) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	clientset, err := inclusterConfig()
	if err != nil {
		panic(err.Error())
	}
	podList, err := clientset.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("error getting pods: %v\n", err)
		os.Exit(1)
	}
	return len(podList.Items), nil
}

func serviceCounter(namespace string) (int, error) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}
	clientset, err := inclusterConfig()
	if err != nil {
		panic(err.Error())
	}
	serviceList, err := clientset.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("error getting services: %v\n", err)
		os.Exit(1)
	}
	return len(serviceList.Items), nil
}

func pvcCounter(namespace string) (int, error) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "persistentvolumeclaims",
	}
	clientset, err := inclusterConfig()
	if err != nil {
		panic(err.Error())
	}
	pvcList, err := clientset.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("error getting persistentvolumeclaims: %v\n", err)
		os.Exit(1)
	}
	return len(pvcList.Items), nil
}
