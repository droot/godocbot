package pullrequest

import (
	"context"
	"log"
	"time"

	"github.com/droot/godocbot/pkg/apis/code/v1alpha1"
	"github.com/google/go-github/github"
	"github.com/kubernetes-sigs/controller-runtime/pkg/client"
	"github.com/kubernetes-sigs/controller-runtime/pkg/controller"
	"github.com/kubernetes-sigs/controller-runtime/pkg/handler"
	"github.com/kubernetes-sigs/controller-runtime/pkg/manager"
	"github.com/kubernetes-sigs/controller-runtime/pkg/reconcile"
	"github.com/kubernetes-sigs/controller-runtime/pkg/source"
	"k8s.io/apimachinery/pkg/api/errors"
)

type GithubSyncer struct {
	mgr      manager.Manager
	ctrl     controller.Controller
	ghClient *github.Client
}

func NewGithubSyncer(mgr manager.Manager) (*GithubSyncer, error) {
	ghClient := github.NewClient(nil)
	ctrl, err := controller.New(
		"github-pullrequest-syncer",
		mgr,
		controller.Options{
			Reconcile: &GHPullRequestReconciler{
				Client:   mgr.GetClient(),
				ghClient: ghClient,
			},
		})
	if err != nil {
		return nil, err
	}
	syncer := &GithubSyncer{
		mgr:      mgr,
		ctrl:     ctrl,
		ghClient: ghClient,
	}

	// Watch PullRequests objects
	if err := syncer.ctrl.Watch(
		&source.Kind{Type: &v1alpha1.PullRequest{}},
		&handler.Enqueue{}); err != nil {
		return nil, err
	}
	// watch the pull requests here
	return syncer, nil
}

func (gs *GithubSyncer) Start(stop <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-stop:
			/* we got signalled */
			return
		case <-ticker.C:
			gs.syncPullRequests()
		}
	}
}

func (gs *GithubSyncer) syncPullRequests() {
	prList := &v1alpha1.PullRequestList{}
	// get all the pull request CRs from k8s
	err := gs.mgr.GetClient().List(context.Background(), &client.ListOptions{Namespace: ""}, prList)
	if err != nil {
		log.Printf("error fetching all the PRs from k8s: %v", err)
		return
	}

	for _, pr := range prList.Items {
		if err = gs.syncPR(&pr); err != nil {
			log.Printf("error syncing pr: %v", err)
		}
	}
}

func (gs *GithubSyncer) syncPR(pr *v1alpha1.PullRequest) error {
	log.Printf("syncing pr: %s/%v", pr.Namespace, pr.Name)
	return nil
}

type GHPullRequestReconciler struct {
	Client   client.Client
	ghClient *github.Client
}

func (r *GHPullRequestReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()

	log.Printf("reconciling request '%v'", request.NamespacedName)
	// Fetch PullRequest object
	pr := &v1alpha1.PullRequest{}
	err := r.Client.Get(ctx, request.NamespacedName, pr)
	if errors.IsNotFound(err) {
		log.Printf("Could not find PullRequest %v.\n", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		log.Printf("Could not fetch PullRequest %v for %+v\n", err, request)
		return reconcile.Result{}, err
	}

	if pr.Spec.CommitID != "" {
		// figure out the latest commit-id
		return reconcile.Result{}, nil
	}

	prinfo, err := parsePullRequestURL(pr.Spec.URL)
	if err != nil {
		log.Printf("error in parsing the request, ignoring this pr %v err %v", request.NamespacedName, err)
		return reconcile.Result{}, nil
	}

	log.Printf("fetching commit id for the PR: %v", prinfo)
	ghPR, _, err := r.ghClient.PullRequests.Get(context.Background(), prinfo.org, prinfo.repo, int(prinfo.pr))
	if err != nil {
		log.Printf("error fetching PR details from github: %v", err)
		return reconcile.Result{}, err
	}

	// deep copy
	prCopy := pr.DeepCopy()
	prCopy.Spec.CommitID = ghPR.Head.GetSHA()
	err = r.Client.Update(context.Background(), prCopy)
	if err != nil {
		log.Printf("error updating PR github: %v", err)
		return reconcile.Result{}, err
	}

	// reconcile the dp with pr now.
	// Print the ReplicaSet
	return reconcile.Result{}, nil
}
