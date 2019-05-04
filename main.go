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

	if pod.ObjectMeta.Annotations["webhook.k8s.macpaw.io/skip"] == "true" {
		return false, nil
	} else {
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

	logger := &log.Std{Debug: viper.GetBool("debug")}

	podHandler := handlerFor(mutating.WebhookConfig{Name: "webhook-admission-mutation", Obj: &corev1.Pod{}}, mutating.MutatorFunc(mutator), logger)

	mux := http.NewServeMux()
	mux.Handle("/pods", podHandler)

	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 3000 
	}

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error serving webhook: %s", err)
		os.Exit(1)
	}
}
