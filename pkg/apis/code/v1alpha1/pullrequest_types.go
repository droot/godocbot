package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PullRequestSpec defines the desired state of PullRequest
type PullRequestSpec struct {
	// URL of the PULL Request
	URL string `json:"url"`

	// Latest commit ID on the PR. This is optional.
	CommitID string `json:"commit_id, omitempty"`
}

// PullRequestStatus defines the observed state of PullRequest
type PullRequestStatus struct {
	// The URL which is serving the godoc for the PR.
	GoDocLink string `json:"godoc_link"`

	// CommitID for which the godoc is being served
	CommitID string `json:"commit_id"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PullRequest
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=pullrequests
type PullRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PullRequestSpec   `json:"spec,omitempty"`
	Status PullRequestStatus `json:"status,omitempty"`
}
