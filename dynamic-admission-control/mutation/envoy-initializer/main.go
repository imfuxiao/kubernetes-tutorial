package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ghodss/yaml"
	"io/ioutil"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

const (
	defaultAnnotation      = "initializer.kubernetes.io/envoy"
	defaultInitializerName = "envoy.initializer.kubernetes.io"
	defaultConfigmap       = "envoy-initializer"
	defaultNamespace       = "envoy"
	defaultKubeConfigPath  = "/Users/morse/.kube/config"
)

var (
	annotation        string
	configmap         string
	initializerName   string
	namespace         string
	requireAnnotation bool
	kubeConfigPath    string
)

type AdmitFunc func(review admissionv1.AdmissionReview) *admissionv1.AdmissionResponse

type httpsConfig struct {
	CertFile string
	KeyFile  string
}

func (c *httpsConfig) addFlags() {
	fmt.Println(flag.Args())
	flag.StringVar(&c.CertFile, "tls-cert-file", c.CertFile, "File containing the default x509 Certificate for https.")
	flag.StringVar(&c.KeyFile, "tls-private-key-file", c.KeyFile, "File containing the default x509 Certificate for https.")
}

func (c *httpsConfig) configTLS() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

type config struct {
	Containers []corev1.Container
	Volumes    []corev1.Volume
}

func main() {
	klog.InitFlags(nil)
	_ = flag.Set("logtostderr", "true")

	httpCfg := &httpsConfig{}
	httpCfg.addFlags()

	flag.StringVar(&annotation, "annotation", defaultAnnotation, "The annotation to trigger initialization")
	flag.StringVar(&configmap, "configmap", defaultConfigmap, "The envoy initializer configuration configmap")
	flag.StringVar(&initializerName, "initializer-name", defaultInitializerName, "The initializer name")
	flag.StringVar(&namespace, "namespace", defaultNamespace, "The configuration namespace")
	flag.BoolVar(&requireAnnotation, "require-annotation", false, "Require annotation for initialization")
	flag.StringVar(&kubeConfigPath, "kube-config-path", defaultKubeConfigPath, "Kubernetes config for initialization")

	flag.Parse()

	// 获取命令行参数
	klog.Infof("arguments: %d", len(os.Args))
	for k, v := range os.Args {
		klog.Infof("args[%v]=[%v]\n", k, v)
	}

	klog.Info("Starting the Kubernetes initializer...")
	klog.Infof("Initializer name set to: %s", initializerName)
	klog.Infof("Initializer CertFile set to: %s", httpCfg.CertFile)
	klog.Infof("Initializer KeyFile set to: %s", httpCfg.KeyFile)
	klog.Infof("Initializer kubeConfigPath set to: %s", kubeConfigPath)

	// 1. 从容器中获取config配置
	clusterConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Warningf("rest.InClusterConfig() error %+v", err)
		clusterConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			klog.Fatalf("clientcmd.BuildConfigFromFlags error %+v", err)
		}
	}

	// 2. 实例化ClientSet
	clientset, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		klog.Fatalf("kubernetes.NewForConfig() error %+v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init httpServer
	tlsConfig, err := httpCfg.configTLS()
	if err != nil {
		klog.Fatalf("tls config error %+v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/envoy-init", func(resp http.ResponseWriter, req *http.Request) {
		server(resp, req, func(review admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {

			if except, actual := "pods", review.Request.Resource.Resource; except != actual {
				err := fmt.Errorf("unexcepted resource: expect %s, actual %s", except, actual)
				klog.Error(err)
				return toAdmissionResponse(true, err)
			}

			// 1. 获取envoy的configmap配置
			klog.Infof("get envoy configmap")
			cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configmap, metav1.GetOptions{})
			if err != nil {
				klog.Error("clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configmap, metav1.GetOptions{}) error:%+v", err)
				return toAdmissionResponse(true, nil)
			}
			klog.Infof("get envoy configmap:%+v", cm)

			// 2. 将yaml转为config
			c, err := configmapToConfig(cm)
			if err != nil {
				klog.Fatalf("configmapToConfig() error:%+v", err)
			}

			pod := corev1.Pod{}
			deserializer := codecs.UniversalDeserializer()
			if _, _, err := deserializer.Decode(review.Request.Object.Raw, nil, &pod); err != nil {
				klog.Error(err)
				return toAdmissionResponse(true, err)
			}

			pod.Spec.Containers = append(pod.Spec.Containers, c.Containers...)
			pod.Spec.Volumes = append(pod.Spec.Volumes, c.Volumes...)

			var po []*PatchOperation

			if len(c.Containers) > 0 {
				po = append(po, &PatchOperation{
					Op:    "add",
					Path:  "/spec/containers",
					Value: pod.Spec.Containers,
				})
			}

			if len(c.Volumes) > 0 {
				po = append(po, &PatchOperation{
					Op:    "add",
					Path:  "/spec/volumes",
					Value: pod.Spec.Volumes,
				})
			}

			resp := toAdmissionResponse(true, nil)
			if len(po) > 0 {
				pathBytes, err := json.Marshal(po)
				if err != nil {
					klog.Error(err)
					return toAdmissionResponse(true, err)
				}
				klog.Infof("Patch: %s", string(pathBytes))
				resp.Patch = pathBytes
				patchType := admissionv1.PatchTypeJSONPatch
				resp.PatchType = &patchType
			}

			return resp
		})
	})

	svc := &http.Server{
		Addr:      ":443",
		TLSConfig: tlsConfig,
		Handler:   mux,
	}

	go func() {
		klog.Fatalln(svc.ListenAndServeTLS(httpCfg.CertFile, httpCfg.KeyFile))
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	klog.Infoln("Shutdown signal recived, exiting...")
}

func server(resp http.ResponseWriter, req *http.Request, admitFunc AdmitFunc) {

	klog.Info("received request: %+v", req)

	var body []byte
	if req.Body != nil {
		if data, err := ioutil.ReadAll(req.Body); err == nil {
			body = data
		}
	}

	// 验证httpRequest内容
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		klog.Errorf("content-type=%s, expect application/json", contentType)
		return
	}

	// requestAdmissionReview
	requestAdmissionReview, responseAdmissionReview := admissionv1.AdmissionReview{}, admissionv1.AdmissionReview{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &requestAdmissionReview); err != nil {
		klog.Errorf("deserializer.Decode() error:%+v", err)
		responseAdmissionReview.Response = toAdmissionResponse(false, err)
	} else {
		responseAdmissionReview.Response = admitFunc(requestAdmissionReview)
	}

	responseAdmissionReview.Response.UID = requestAdmissionReview.Request.UID
	responseAdmissionReview.APIVersion = admissionv1.SchemeGroupVersion.String()
	responseAdmissionReview.Kind = "AdmissionReview"

	//klog.Info(fmt.Sprintf("sending response: %+v", responseAdmissionReview.Response))

	respBytes, err := json.Marshal(responseAdmissionReview)
	if err != nil {
		klog.Errorf("json.Marshal() error:%+v", err)
	}

	if _, err := resp.Write(respBytes); err != nil {
		klog.Errorf("resp write error: %+v", err)
	}
}

func toAdmissionResponse(allowed bool, err error) *admissionv1.AdmissionResponse {
	response := &admissionv1.AdmissionResponse{
		Allowed: allowed,
	}
	if err != nil {
		response.Result = &metav1.Status{
			Message: err.Error(),
		}
	}
	return response
}

func configmapToConfig(cm *corev1.ConfigMap) (*config, error) {
	var c config
	return &c, yaml.Unmarshal([]byte(cm.Data["config"]), &c)
}
