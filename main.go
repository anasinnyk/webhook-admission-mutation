package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	whhttp "github.com/slok/kubewebhook/pkg/http"
	"github.com/slok/kubewebhook/pkg/log"
	"github.com/slok/kubewebhook/pkg/webhook/mutating"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mutator(ctx context.Context, obj metav1.Object) (bool, error) {
	var pod *corev1.Pod

	switch v := obj.(type) {
	case *corev1.Pod:
		pod = v
	default:
		return false, nil
	}

	fmt.Printf("%v\n", pod.ObjectMeta.Annotations)
	if pod.ObjectMeta.Annotations["webhook.k8s.macpaw.io/skip"] == "true" {
		return false, nil
	} else {
		if pod.ObjectMeta.Annotations["webhook.k8s.macpaw.io/sc-100"] == "true" {
			RunAsUser := int64(100)
			pod.Spec.Containers[0].SecurityContext.RunAsUser = &RunAsUser
		}

		if pod.ObjectMeta.Annotations["webhook.k8s.macpaw.io/cmd"] == "true" {
			pod.Spec.Containers[0].Command = []string{"echo", "123"}
		}

		if pod.ObjectMeta.Annotations["webhook.k8s.macpaw.io/init-container"] == "true" {
			allowPrivilegeEscalation := false
			securityContext := &corev1.SecurityContext{}
			if pod.ObjectMeta.Annotations["webhook.k8s.macpaw.io/dissable-allow-privilege-escalation"] == "true" {
				securityContext.AllowPrivilegeEscalation = &allowPrivilegeEscalation
			}
			pod.Spec.InitContainers = []corev1.Container{
				{
					Name: "ubuntu-init",
					Image: "ubuntu",
					Command: []string{"echo", "init"},
					SecurityContext: securityContext,
				},
			}
		}

		if pod.ObjectMeta.Annotations["webhook.k8s.macpaw.io/volume"] == "true" {
			pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					Name:      "vault-env",
					MountPath: "/vault/",
				},
			}

			pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
				Name: "vault-env",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			})
		}

		pod.ObjectMeta.Annotations["webhook.k8s.macpaw.io/mutated"] = "true"
	}

	return false, nil
}

func handlerFor(config mutating.WebhookConfig, mutator mutating.MutatorFunc, logger log.Logger) http.Handler {
	webhook, err := mutating.NewWebhook(config, mutator, nil, nil, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating webhook: %s", err)
		os.Exit(1)
	}

	handler, err := whhttp.HandlerFor(webhook)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating webhook: %s", err)
		os.Exit(1)
	}

	return handler
}

func main() {
	viper.AutomaticEnv()
	logger := &log.Std{Debug: viper.GetBool("debug")}
	podHandler := handlerFor(mutating.WebhookConfig{Name: "webhook-admission-mutation", Obj: &corev1.Pod{}}, mutating.MutatorFunc(mutator), logger)

	mux := http.NewServeMux()
	mux.Handle("/pods", podHandler)

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 8443 
	}

	err = http.ListenAndServeTLS(fmt.Sprintf(":%d", port), viper.GetString("tls_cert_file"), viper.GetString("tls_private_key_file"), mux)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error serving webhook: %s", err)
		os.Exit(1)
	}
}
