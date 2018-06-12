package inject

import (
	codev1alpha1 "github.com/droot/godocbot/pkg/apis/code/v1alpha1"
	rscheme "github.com/droot/godocbot/pkg/client/clientset/versioned/scheme"
	"github.com/droot/godocbot/pkg/controller/pullrequest"
	"github.com/droot/godocbot/pkg/inject/args"
	"github.com/kubernetes-sigs/kubebuilder/pkg/inject/run"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	rscheme.AddToScheme(scheme.Scheme)

	// Inject Informers
	Inject = append(Inject, func(arguments args.InjectArgs) error {
		Injector.ControllerManager = arguments.ControllerManager

		if err := arguments.ControllerManager.AddInformerProvider(&codev1alpha1.PullRequest{}, arguments.Informers.Code().V1alpha1().PullRequests()); err != nil {
			return err
		}

		// Add Kubernetes informers

		if c, err := pullrequest.ProvideController(arguments); err != nil {
			return err
		} else {
			arguments.ControllerManager.AddController(c)
		}
		return nil
	})

	// Inject CRDs
	Injector.CRDs = append(Injector.CRDs, &codev1alpha1.PullRequestCRD)
	// Inject PolicyRules
	Injector.PolicyRules = append(Injector.PolicyRules, rbacv1.PolicyRule{
		APIGroups: []string{"code.godocs.io"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	})
	// Inject GroupVersions
	Injector.GroupVersions = append(Injector.GroupVersions, schema.GroupVersion{
		Group:   "code.godocs.io",
		Version: "v1alpha1",
	})
	Injector.RunFns = append(Injector.RunFns, func(arguments run.RunArguments) error {
		Injector.ControllerManager.RunInformersAndControllers(arguments)
		return nil
	})
}
