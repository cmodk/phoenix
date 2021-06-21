package main

import (
	"errors"
	"log"
	"strings"

	//appsv1 "k8s.io/api/apps/v1"
	//apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	//"k8s.io/client-go/util/retry"
)

func k8sDeleteAllPods() error {
	if len(namespace_arg) == 0 {
		return errors.New("Missing namespace arg")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	podClient := clientset.CoreV1().Pods("phoenix-" + namespace_arg)

	pods, err := podClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	log.Println("Listing pods")
	for _, pod := range pods.Items {
		log.Println(pod.Name)
		deleteOptions := metav1.DeleteOptions{}
		podClient.Delete(pod.Name, &deleteOptions)
	}

	return nil
}

func k8sRecreatePods() error {
	if len(namespace_arg) == 0 {
		return errors.New("Missing namespace arg")
	}

	if len(podname_arg) == 0 {
		return errors.New("Missing pod name")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	podClient := clientset.CoreV1().Pods(namespace_arg)

	pods, err := podClient.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	log.Println("Listing pods")
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, podname_arg) {
			log.Println(pod.Name)
			deleteOptions := metav1.DeleteOptions{}
			podClient.Delete(pod.Name, &deleteOptions)
		}
	}

	return nil
}
