package pullrequest

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/droot/godocbot/pkg/apis/code/v1alpha1"
	"github.com/kubernetes-sigs/controller-runtime/pkg/client"
	"github.com/kubernetes-sigs/controller-runtime/pkg/controller/reconcile"
)

type PullRequestReconciler struct {
	Client client.Interface
}

func (r *PullRequestReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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

	if pr.Status.GoDocLink == "" && dp.Status.AvailableReplicas > 0 {
		log.Printf("deployment became available, updating the godoc link")
		// update the status
		prinfo, _ := parsePullRequestURL(pr.Spec.URL)
		pr.Status.GoDocLink = fmt.Sprintf("https://%s.serveo.net/pkg/%s/%s/%s", prinfo.subdomain(), prinfo.host, prinfo.org, prinfo.repo)
		if err = r.Client.Update(ctx, pr); err != nil {
			return reconcile.Result{}, err
		}
		log.Printf("godoc link updated successfully for pr %v", request.NamespacedName)
	}

	// reconcile the dp with pr now.
	// Print the ReplicaSet
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
							Name:            "ttyd",
							ImagePullPolicy: "Always",
							Command:         []string{"/bin/bash"},
							Args:            []string{"fetch_serve.sh", prinfo.host, prinfo.org, prinfo.repo, prinfo.pr},
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
	host string
	org  string
	repo string
	pr   string
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

	return &prInfo{
		host: u.Hostname(),
		org:  parts[0],
		repo: parts[1],
		pr:   parts[3],
	}, nil

}

// helper function to generate subdomain for the prinfo.
func (pr *prInfo) subdomain() string {
	return fmt.Sprintf("%s-%s-pr-%s", pr.org, pr.repo, pr.pr)
}
