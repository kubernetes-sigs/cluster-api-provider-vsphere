/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"sync"
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	clusterapiclientsetscheme "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/scheme"
	"sigs.k8s.io/cluster-api/pkg/client/informers_generated/externalversions"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager) error
var doOnce sync.Once
var siFactory externalversions.SharedInformerFactory
var siStopper = make(chan struct{})

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m); err != nil {
			glog.Infof("Failed to add to manager:  %s", err.Error())
			return err
		}
	}
	return nil
}

func getSharedInformerFactory(m manager.Manager) externalversions.SharedInformerFactory {
	client, err := clientset.NewForConfig(m.GetConfig())
	if err != nil {
		return nil
	}

	doOnce.Do(func() {
		siFactory = externalversions.NewSharedInformerFactory(client, 10*time.Second)
		// Call informers that we care about so they can be monitored when we call Start() below.
		// The generated factory code doesn't mention this, but it's necessary.
		siFactory.Cluster().V1alpha1().Clusters().Informer()
		siFactory.Cluster().V1alpha1().Machines().Informer()
		siFactory.Start(siStopper)
	})

	return siFactory
}

func createRecorder(kubeClient *kubernetes.Clientset, source string) (record.EventRecorder, error) {
	eventsScheme := runtime.NewScheme()
	if err := corev1.AddToScheme(eventsScheme); err != nil {
		return nil, err
	}
	// We also emit events for our own types.
	clusterapiclientsetscheme.AddToScheme(eventsScheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(eventsScheme, corev1.EventSource{Component: source}), nil
}
