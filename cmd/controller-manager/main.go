package main

import (
	"flag"
	"log"

	// Import auth/gcp to connect to GKE clusters remotely
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/droot/godocbot/pkg/apis/code/v1alpha1"
	"github.com/droot/godocbot/pkg/controller/pullrequest"
	"github.com/kubernetes-sigs/controller-runtime/pkg/client/config"
	"github.com/kubernetes-sigs/controller-runtime/pkg/manager"
	logf "github.com/kubernetes-sigs/controller-runtime/pkg/runtime/log"
	"github.com/kubernetes-sigs/controller-runtime/pkg/runtime/signals"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var enablePRSync = flag.Bool("enable-pr-sync", false, "if set to true, periodically syncs pullrequest with Github")

// Controller-manager main.
func main() {
	flag.Parse()
	logf.SetLogger(logf.ZapLogger(false))

	// Setup a ControllerManager
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		log.Fatal(err)
	}

	// register PullRequest Type with Manager's scheme.
	registerTypes(mgr)

	stop := signals.SetupSignalHandler()

	_, err = pullrequest.NewGodocDeployer(mgr)
	if err != nil {
		log.Fatalf("failed to create godoc deployer: %v", err)
	}

	_, err = pullrequest.NewGithubSyncer(mgr, *enablePRSync, stop)
	if err != nil {
		log.Fatalf("failed to create the github pull request syncer %v", err)
	}

	// start the manager
	log.Fatal(mgr.Start(stop))
}

func registerTypes(mgr manager.Manager) {
	mgr.GetScheme().AddKnownTypes(v1alpha1.SchemeGroupVersion, &v1alpha1.PullRequest{}, &v1alpha1.PullRequestList{})
	metav1.AddToGroupVersion(mgr.GetScheme(), v1alpha1.SchemeGroupVersion)
}
