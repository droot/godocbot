package pullrequest

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"github.com/droot/godocbot/pkg/apis/code/v1alpha1"
	"github.com/kubernetes-sigs/controller-runtime/pkg/client"
	"github.com/kubernetes-sigs/controller-runtime/pkg/controller"
	"github.com/kubernetes-sigs/controller-runtime/pkg/handler"
	"github.com/kubernetes-sigs/controller-runtime/pkg/manager"
	"github.com/kubernetes-sigs/controller-runtime/pkg/reconcile"
	"github.com/kubernetes-sigs/controller-runtime/pkg/source"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GodocDeployer watches PullRequest object which have a commitID specified in
// their Spec and deploys a Godoc deployment which runs godoc server for the PR.
// It watches the PullRequest object for changes in commitID and reconciles the
// generated godoc deployment.
type GodocDeployer struct {
	controller.Controller
}

func NewGodocDeployer(mgr manager.Manager) (*GodocDeployer, error) {
	prReconciler := &pullRequestReconciler{
		Client: mgr.GetClient(),
	}

	// Setup a new controller to Reconcile PullRequests
	c, err := controller.New("pull-request-controller", mgr, controller.Options{Reconcile: prReconciler})
	if err != nil {
		return nil, err
	}

	// Watch PullRequest objects
	err = c.Watch(
		&source.Kind{Type: &v1alpha1.PullRequest{}},
		&handler.Enqueue{})
	if err != nil {
		return nil, err
	}

	// Watch deployments generated for PullRequests objects
	err = c.Watch(
		&source.Kind{Type: &appsv1.Deployment{}},
		&handler.EnqueueOwner{
			OwnerType:    &v1alpha1.PullRequest{},
			IsController: true,
		},
	)
	if err != nil {
		return nil, err
	}

	return &GodocDeployer{Controller: c}, nil
}

// pullRequestReconciler ensures there is a godoc deployment is running with
// the commitID specified in the PullRequest object.
type pullRequestReconciler struct {
	Client client.Client
}

func (r *pullRequestReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

	if pr.Spec.CommitID == "" {
		log.Printf("Waiting for PR %s commitID to be updated", request.NamespacedName)
		return reconcile.Result{}, nil
	}

	dp := &appsv1.Deployment{}
	err = r.Client.Get(ctx, request.NamespacedName, dp)
	if errors.IsNotFound(err) {
		log.Printf("Could not find deployment for PullRequest %v. creating deployment", request)

		dp, err = deploymentForPullRequest(pr)
		if err != nil {
			log.Printf("error creating new deployment for PullRequest: %v %v", request, err)
			return reconcile.Result{}, nil
		}
		if err = r.Client.Create(ctx, dp); err != nil {
			log.Printf("error creating new deployment for PullRequest: %v %v", request, err)
			return reconcile.Result{}, err
		}
		log.Printf("created deployment for %v successfully", request.NamespacedName)
	}

	prinfo, _ := parsePullRequestURL(pr.Spec.URL)
	prinfo.commitID = pr.Spec.CommitID

	if len(dp.Spec.Template.Spec.Containers[0].Args) <= 5 || (pr.Spec.CommitID != dp.Spec.Template.Spec.Containers[0].Args[5]) {
		// deployment is not updated with latest commit-id
		dpCopy := dp.DeepCopy()
		dpCopy.Spec.Template.Spec.Containers[0].Args = prinfo.godocContainerArgs()
		if err = r.Client.Update(ctx, dpCopy); err != nil {
			log.Printf("error updating the deployment for key %s", request.NamespacedName)
			return reconcile.Result{}, err
		}
	}

	if pr.Status.GoDocLink == "" && dp.Status.AvailableReplicas > 0 {
		log.Printf("deployment became available, updating the godoc link")
		// update the status
		prCopy := pr.DeepCopy()
		prCopy.Status.GoDocLink = fmt.Sprintf("https://%s.serveo.net/pkg/%s/%s/%s", prinfo.subdomain(), prinfo.host, prinfo.org, prinfo.repo)
		if err = r.Client.Update(ctx, prCopy); err != nil {
			return reconcile.Result{}, err
		}
		log.Printf("godoc link updated successfully for pr %v", request.NamespacedName)
	}
	return reconcile.Result{}, nil
}

// deploymentForPullRequest creates a deployment object for a given PullRequest.
func deploymentForPullRequest(pr *v1alpha1.PullRequest) (*appsv1.Deployment, error) {
	// we are good with running with one replica
	var replicas int32 = 1

	prinfo, err := parsePullRequestURL(pr.Spec.URL)
	if err != nil {
		return nil, err
	}
	prinfo.commitID = pr.Spec.CommitID

	labels := map[string]string{
		"org":  prinfo.org,
		"repo": prinfo.repo,
	}

	tunnelArgs := "80:localhost:6060"
	tunnelArgs = fmt.Sprintf("%s:%s", prinfo.subdomain(), tunnelArgs)

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pr.Name,
			Namespace: pr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Image:           "gcr.io/sunilarora-sandbox/godoc:0.0.1",
							Name:            "godoc",
							ImagePullPolicy: "Always",
							Command:         []string{"/bin/bash"},
							Args:            prinfo.godocContainerArgs(),
						},
						{
							Image:           "gcr.io/sunilarora-sandbox/ssh-client:0.0.2",
							Name:            "ssh",
							ImagePullPolicy: "Always",
							Command:         []string{"ssh"},
							Args:            []string{"-tt", "-o", "StrictHostKeyChecking=no", "-R", tunnelArgs, "serveo.net"},
						},
					},
				},
			},
		},
	}
	addOwnerRefToObject(dep, *metav1.NewControllerRef(pr, schema.GroupVersionKind{
		Group:   v1alpha1.SchemeGroupVersion.Group,
		Version: v1alpha1.SchemeGroupVersion.Version,
		Kind:    "PullRequest",
	}))
	return dep, nil
}

// addOwnerRefToObject appends the desired OwnerReference to the object
func addOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}

// prInfo is an structure to represent PullRequest info. It will be used
// internally to more as an convenience for passing it around.
type prInfo struct {
	host     string
	org      string
	repo     string
	pr       int64
	commitID string
}

// parsePullRequestURL parses given PullRequest URL into prInfo instance.
// An example pull request URL looks like:
// https://github.com/kubernetes-sigs/controller-runtime/pull/15
func parsePullRequestURL(prURL string) (*prInfo, error) {
	u, err := url.Parse(prURL)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "pull" {
		return nil, fmt.Errorf("pr info missing in the URL")
	}

	prNum, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, err
	}

	return &prInfo{
		host: u.Hostname(),
		org:  parts[0],
		repo: parts[1],
		pr:   prNum,
	}, nil
}

// helper function to generate subdomain for the prinfo.
func (pr *prInfo) subdomain() string {
	return fmt.Sprintf("%s-%s-pr-%d", pr.org, pr.repo, pr.pr)
}

func (pr *prInfo) godocContainerArgs() []string {
	return []string{"fetch_serve.sh", pr.host, pr.org, pr.repo, strconv.FormatInt(pr.pr, 10), pr.commitID}
}
