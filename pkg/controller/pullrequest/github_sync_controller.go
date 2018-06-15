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
	ticker := time.NewTicker(30 * time.Second)
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

// this function organizes all the PRs in the given list  by repos and orgs,
// so that we can make  batch calls to GH.
// { "kubernetes-sigs":
//		{
//			"controller-runtime": {
//				21: "commit-id",
//
//			}
//		}
//	}
func pullRequestByRepoAndOrg(prs *v1alpha1.PullRequestList) map[string]map[string]map[int64]string {
	orgs := map[string]map[string]map[int64]string{}
	for _, pr := range prs.Items {
		prinfo, _ := parsePullRequestURL(pr.Spec.URL)
		log.Printf("populating %v in the map", prinfo)
		_, orgFound := orgs[prinfo.org]
		if !orgFound {
			orgs[prinfo.org] = map[string]map[int64]string{}
		}
		_, repoFound := orgs[prinfo.org][prinfo.repo]
		if !repoFound {
			orgs[prinfo.org][prinfo.repo] = map[int64]string{}
		}
		orgs[prinfo.org][prinfo.repo][prinfo.pr] = pr.Spec.CommitID
	}
	return orgs
}

func (gs *GithubSyncer) syncPullRequests() {
	prList := &v1alpha1.PullRequestList{}
	// get pull requests in all namespaces
	err := gs.mgr.GetClient().List(context.Background(), &client.ListOptions{Namespace: ""}, prList)
	if err != nil {
		log.Printf("error fetching all the PRs from k8s: %v", err)
		return
	}

	orgs := pullRequestByRepoAndOrg(prList)
	log.Printf("orgs map: %+v", orgs)

	for org, repos := range orgs {
		for repo, prs := range repos {
			// for pr, commitID := range prs {
			// 	log.Printf("org: %s repo:%s pr: %d commitID: %s \n", org, repo, pr, commitID)
			// }
			ghPRs, _, err := gs.ghClient.PullRequests.List(context.Background(), org, repo, nil)
			if err != nil {
				log.Printf("cannont get PR list from GH: %v", err)
				continue
			}
			for _, ghPR := range ghPRs {
				ghPRNum := int64(ghPR.GetNumber())
				if commitID, found := prs[ghPRNum]; found {
					// github PR found in our cluster
					if ghPR.Head.GetSHA() != commitID {
						// PR has been updated in GitHub
						log.Printf("PR Updated: org: %s repo:%s pr: %d commitID: %s ghCommitID: %s \n", org, repo, ghPRNum, commitID, ghPR.Head.GetSHA())
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

	// for _, pr := range prList.Items {
	// 	if err = gs.syncPR(&pr); err != nil {
	// 		log.Printf("error syncing pr: %v", err)
	// 	}
	// }
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

	// deep copy
	prCopy := pr.DeepCopy()
	prCopy.Spec.CommitID = ghPR.Head.GetSHA()
	err = r.Client.Update(context.Background(), prCopy)
	if err != nil {
		log.Printf("error updating PR github: %v", err)
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
