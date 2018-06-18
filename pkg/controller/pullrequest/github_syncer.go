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

// GithubSyncer implements following functionalities:
//  - Watches newly created PRs in K8s and updates their commitID by calling
//  Github
//  - Periodically updates the PRs in K8s with their commitID in Github.
type GithubSyncer struct {
	mgr  manager.Manager
	ctrl controller.Controller
	// TODO(droot): take make GithubClient interface to make it easy to test the
	// syncer
	ghClient *github.Client
	// TODO(droot): parameterize the sync duration
	syncInterval time.Duration
}

func NewGithubSyncer(mgr manager.Manager, enablePRSync bool, stop <-chan struct{}) (*GithubSyncer, error) {
	ghClient := github.NewClient(nil)
	ctrl, err := controller.New(
		"github-pullrequest-syncer",
		mgr,
		controller.Options{
			Reconcile: &pullRequestCommitIDReconciler{
				Client:   mgr.GetClient(),
				ghClient: ghClient,
			},
		})
	if err != nil {
		return nil, err
	}
	syncer := &GithubSyncer{
		mgr:          mgr,
		ctrl:         ctrl,
		ghClient:     ghClient,
		syncInterval: 30 * time.Second,
	}

	// Watch PullRequests objects
	if err := syncer.ctrl.Watch(
		&source.Kind{Type: &v1alpha1.PullRequest{}},
		&handler.Enqueue{}); err != nil {
		return nil, err
	}

	if enablePRSync {
		go syncer.Start(stop)
	}
	return syncer, nil
}

// pullRequestCommitIDReconciler reconciles commitID of newly created
// pullrequests in K8s.
type pullRequestCommitIDReconciler struct {
	Client client.Client
	// TODO(droot): take GithubClient interface to improve testability
	ghClient *github.Client
}

func (r *pullRequestCommitIDReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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
		// We already know the commit-id for the PR, so no need to update the commitID
		return reconcile.Result{}, nil
	}

	// Looks like this is a fresh PR, so lets determine the latest commitID
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

	// deep copy ? check if it is still required with pkg/cache or client ?
	prCopy := pr.DeepCopy()
	prCopy.Spec.CommitID = ghPR.Head.GetSHA()
	err = r.Client.Update(context.Background(), prCopy)
	if err != nil {
		log.Printf("error updating PR github: %v", err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// Start periodically syncs PRs in k8s with their commitID in Github.
func (gs *GithubSyncer) Start(stop <-chan struct{}) {
	ticker := time.NewTicker(gs.syncInterval)
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

// syncPullRequests performs the following:
//  - fetches all the PRs registered in K8s
//  - Organize them by orgs/repo so that batch calls can be made
//  - Fetches details of PRs from Github and updates the commitID of the PRs in
//  k8s if required.
// TODO(droot): delete the PRs from K8s if closed in Github.
func (gs *GithubSyncer) syncPullRequests() {
	prList := &v1alpha1.PullRequestList{}
	// get pull requests in all namespaces
	err := gs.mgr.GetClient().List(context.Background(), &client.ListOptions{Namespace: ""}, prList)
	if err != nil {
		log.Printf("error fetching all the PRs from k8s: %v", err)
		return
	}

	// TODO(droot): simplify the following loop
	orgs := pullRequestByRepoAndOrg(prList)
	for org, repos := range orgs {
		for repo, prs := range repos {
			ghPRs, _, err := gs.ghClient.PullRequests.List(context.Background(), org, repo, nil)
			if err != nil {
				log.Printf("cannont get PR list from GH: %v", err)
				continue
			}
			for _, ghPR := range ghPRs {
				ghPRNum := int64(ghPR.GetNumber())
				if pr, found := prs[ghPRNum]; found {
					commitID := pr.Spec.CommitID
					// github PR found in our cluster
					if ghPR.Head.GetSHA() != commitID {
						// PR has been updated in GitHub
						log.Printf("PR Updated: org: %s repo:%s pr: %d commitID: %s ghCommitID: %s \n", org, repo, ghPRNum, commitID, ghPR.Head.GetSHA())
						pr.Spec.CommitID = ghPR.Head.GetSHA()
						err = gs.mgr.GetClient().Update(context.Background(), pr)
						if err != nil {
							log.Printf("error updating PR github: %v", err)
							continue
						}
					} else {
						log.Printf("PR is same: org: %s repo:%s pr: %d commitID: %s ghCommitID: %s \n", org, repo, ghPRNum, commitID, ghPR.Head.GetSHA())
					}
				} else {
					log.Printf("PR not found: org: %s repo:%s pr: %d \n", org, repo, ghPRNum)
				}
			}
			// TODO(droot): if a PR is not found in GH, it is closed, so we need to probably
			// delete that PR from k8s cluster
		}
	}
}

// this function organizes all the PRs present in K8s by repo names and orgs, so that
// we can make batch calls to Github. It returns map organized as follows:
//   {
//		"org-1": {
//			"repo-1": {
//				pull-request-number-1: "commit-id-1",
//				pull-request-number-2: "commit-id-2",
//				...other pull request to follow...
//			},
//			"repo-2": {
//				pull-request-number-1: "commit-id-1",
//				pull-request-number-2: "commit-id-2",
//				...other pull request to follow...
//			}
//			....other repos to follow....
//		}
//	    ....other orgs to follow....
//	}
// TODO(droot): explore if indexing available under pkg/cache can help us achive
// this organization.
func pullRequestByRepoAndOrg(prs *v1alpha1.PullRequestList) map[string]map[string]map[int64]*v1alpha1.PullRequest {
	orgs := map[string]map[string]map[int64]*v1alpha1.PullRequest{}
	for _, pr := range prs.Items {
		prinfo, _ := parsePullRequestURL(pr.Spec.URL)
		_, orgFound := orgs[prinfo.org]
		if !orgFound {
			orgs[prinfo.org] = map[string]map[int64]*v1alpha1.PullRequest{}
		}
		_, repoFound := orgs[prinfo.org][prinfo.repo]
		if !repoFound {
			orgs[prinfo.org][prinfo.repo] = map[int64]*v1alpha1.PullRequest{}
		}
		orgs[prinfo.org][prinfo.repo][prinfo.pr] = &pr
	}
	return orgs
}
