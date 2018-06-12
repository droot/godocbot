// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/droot/godocbot/pkg/apis/code/v1alpha1"
	scheme "github.com/droot/godocbot/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// PullRequestsGetter has a method to return a PullRequestInterface.
// A group's client should implement this interface.
type PullRequestsGetter interface {
	PullRequests(namespace string) PullRequestInterface
}

// PullRequestInterface has methods to work with PullRequest resources.
type PullRequestInterface interface {
	Create(*v1alpha1.PullRequest) (*v1alpha1.PullRequest, error)
	Update(*v1alpha1.PullRequest) (*v1alpha1.PullRequest, error)
	UpdateStatus(*v1alpha1.PullRequest) (*v1alpha1.PullRequest, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.PullRequest, error)
	List(opts v1.ListOptions) (*v1alpha1.PullRequestList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PullRequest, err error)
	PullRequestExpansion
}

// pullRequests implements PullRequestInterface
type pullRequests struct {
	client rest.Interface
	ns     string
}

// newPullRequests returns a PullRequests
func newPullRequests(c *CodeV1alpha1Client, namespace string) *pullRequests {
	return &pullRequests{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the pullRequest, and returns the corresponding pullRequest object, and an error if there is any.
func (c *pullRequests) Get(name string, options v1.GetOptions) (result *v1alpha1.PullRequest, err error) {
	result = &v1alpha1.PullRequest{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("pullrequests").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PullRequests that match those selectors.
func (c *pullRequests) List(opts v1.ListOptions) (result *v1alpha1.PullRequestList, err error) {
	result = &v1alpha1.PullRequestList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("pullrequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested pullRequests.
func (c *pullRequests) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("pullrequests").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a pullRequest and creates it.  Returns the server's representation of the pullRequest, and an error, if there is any.
func (c *pullRequests) Create(pullRequest *v1alpha1.PullRequest) (result *v1alpha1.PullRequest, err error) {
	result = &v1alpha1.PullRequest{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("pullrequests").
		Body(pullRequest).
		Do().
		Into(result)
	return
}

// Update takes the representation of a pullRequest and updates it. Returns the server's representation of the pullRequest, and an error, if there is any.
func (c *pullRequests) Update(pullRequest *v1alpha1.PullRequest) (result *v1alpha1.PullRequest, err error) {
	result = &v1alpha1.PullRequest{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("pullrequests").
		Name(pullRequest.Name).
		Body(pullRequest).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *pullRequests) UpdateStatus(pullRequest *v1alpha1.PullRequest) (result *v1alpha1.PullRequest, err error) {
	result = &v1alpha1.PullRequest{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("pullrequests").
		Name(pullRequest.Name).
		SubResource("status").
		Body(pullRequest).
		Do().
		Into(result)
	return
}

// Delete takes name of the pullRequest and deletes it. Returns an error if one occurs.
func (c *pullRequests) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("pullrequests").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *pullRequests) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("pullrequests").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched pullRequest.
func (c *pullRequests) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.PullRequest, err error) {
	result = &v1alpha1.PullRequest{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("pullrequests").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}