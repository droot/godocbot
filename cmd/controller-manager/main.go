package main

import (
	"flag"
	"log"

	// Import auth/gcp to connect to GKE clusters remotely
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/droot/godocbot/pkg/apis/code/v1alpha1"
	"github.com/droot/godocbot/pkg/controller/pullrequest"
	"github.com/kubernetes-sigs/controller-runtime/pkg/controller"
	"github.com/kubernetes-sigs/controller-runtime/pkg/controller/eventhandler"
	"github.com/kubernetes-sigs/controller-runtime/pkg/controller/source"
	logf "github.com/kubernetes-sigs/controller-runtime/pkg/runtime/log"
	"github.com/kubernetes-sigs/controller-runtime/pkg/runtime/signals"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// var installCRDs = flag.Bool("install-crds", true, "install the CRDs used by the controller as part of startup")

// Controller-manager main.
func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))

	// Setup a ControllerManager
	manager, err := controller.NewManager(controller.ManagerArgs{})
	if err != nil {
		log.Fatal(err)
	}
	manager.GetScheme().AddKnownTypes(v1alpha1.SchemeGroupVersion, &v1alpha1.PullRequest{}, &v1alpha1.PullRequestList{})
	metav1.AddToGroupVersion(manager.GetScheme(), v1alpha1.SchemeGroupVersion)

	prReconciler := &pullrequest.PullRequestReconciler{
		Client: manager.GetClient(),
	}

	// Setup a new controller to Reconcile PullRequests
	c, err := manager.NewController(
		controller.Args{Name: "pull-request-controller", MaxConcurrentReconciles: 1},
		prReconciler,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Watch PullRequests objects
	err = c.Watch(
		&source.KindSource{Type: &v1alpha1.PullRequest{}},
		&eventhandler.EnqueueHandler{})
	if err != nil {
		log.Fatal(err)
	}

	// Watch deployments generated for PullRequests objects
	err = c.Watch(
		&source.KindSource{Type: &appsv1.Deployment{}},
		&eventhandler.EnqueueOwnerHandler{
			OwnerType:    &appsv1.Deployment{},
			IsController: true,
		},
	)

	log.Fatal(manager.Start(signals.SetupSignalHandler()))

}
