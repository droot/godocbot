package main

import (
	"flag"
	"log"

	// Import auth/gcp to connect to GKE clusters remotely
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/droot/godocbot/pkg/apis/code/v1alpha1"
	"github.com/droot/godocbot/pkg/controller/pullrequest"
	"github.com/kubernetes-sigs/controller-runtime/pkg/client/config"
	"github.com/kubernetes-sigs/controller-runtime/pkg/controller"
	"github.com/kubernetes-sigs/controller-runtime/pkg/handler"
	"github.com/kubernetes-sigs/controller-runtime/pkg/manager"
	logf "github.com/kubernetes-sigs/controller-runtime/pkg/runtime/log"
	"github.com/kubernetes-sigs/controller-runtime/pkg/runtime/signals"
	"github.com/kubernetes-sigs/controller-runtime/pkg/source"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// var installCRDs = flag.Bool("install-crds", true, "install the CRDs used by the controller as part of startup")
var enablePRSync = flag.Bool("enable-pr-sync", false, "if set to true, periodically syncs pull request with Github")

// Controller-manager main.
func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))

	// Setup a ControllerManager
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		log.Fatal(err)
	}
	mgr.GetScheme().AddKnownTypes(v1alpha1.SchemeGroupVersion, &v1alpha1.PullRequest{}, &v1alpha1.PullRequestList{})
	metav1.AddToGroupVersion(mgr.GetScheme(), v1alpha1.SchemeGroupVersion)

	prReconciler := &pullrequest.PullRequestReconciler{
		Client: mgr.GetClient(),
	}

	// Setup a new controller to Reconcile PullRequests
	c, err := controller.New("pull-request-controller", mgr, controller.Options{Reconcile: prReconciler})
	if err != nil {
		log.Fatal(err)
	}

	// Watch PullRequests objects
	err = c.Watch(
		&source.Kind{Type: &v1alpha1.PullRequest{}},
		&handler.Enqueue{})
	if err != nil {
		log.Fatal(err)
	}

	// Watch deployments generated for PullRequests objects
	err = c.Watch(
		&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueOwner{
			OwnerType:    &v1alpha1.PullRequest{},
			IsController: true,
		},
	)

	gs, err := pullrequest.NewGithubSyncer(mgr)
	if err != nil {
		log.Fatalf("failed to create the github pull request syncer %v", err)
	}
	stop := signals.SetupSignalHandler()
	if *enablePRSync {
		go gs.Start(stop)
	}
	log.Fatal(mgr.Start(stop))
}
