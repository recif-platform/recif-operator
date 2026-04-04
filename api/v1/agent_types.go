/*
Copyright 2026 Sciences44.
Licensed under the Apache License, Version 2.0.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentSpec defines the desired state of an Agent
type AgentSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=adk;langchain;crewai;autogen;custom
	Framework string `json:"framework"`

	// +kubebuilder:default=simple
	Strategy string `json:"strategy,omitempty"`

	// +kubebuilder:default=rest
	Channel string `json:"channel,omitempty"`

	// +kubebuilder:default=stub
	ModelType string `json:"modelType,omitempty"`

	// +kubebuilder:default=stub-echo
	ModelID string `json:"modelId,omitempty"`

	SystemPrompt string `json:"systemPrompt,omitempty"`

	// +kubebuilder:default=memory
	// +kubebuilder:validation:Enum=memory;postgresql
	Storage string `json:"storage,omitempty"`

	DatabaseURL string `json:"databaseUrl,omitempty"`

	// List of Tool CRD names assigned to this agent
	// +optional
	Tools []string `json:"tools,omitempty"`

	// Knowledge base IDs for RAG strategy
	// +optional
	KnowledgeBases []string `json:"knowledgeBases,omitempty"`

	// Skill IDs assigned to this agent
	// +optional
	Skills []string `json:"skills,omitempty"`

	// EnvSecrets is a list of Secret names whose data will be injected as env vars.
	// Defaults to ["agent-env"] when empty (backward compatible).
	// +optional
	EnvSecrets []string `json:"envSecrets,omitempty"`

	// CredentialSecret is the name of a Secret containing a GCP/AWS credentials file to mount.
	// Defaults to "gcp-adc" when empty (backward compatible).
	// +optional
	CredentialSecret string `json:"credentialSecret,omitempty"`

	// GCPServiceAccount is the GCP service account email (e.g. "my-agent@project.iam.gserviceaccount.com").
	// When set, the operator looks for a Secret named "{agent}-gcp-sa" with key "credentials.json"
	// containing the service account key, mounts it, and sets GOOGLE_APPLICATION_CREDENTIALS + GOOGLE_CLOUD_PROJECT.
	// +optional
	GCPServiceAccount string `json:"gcpServiceAccount,omitempty"`

	// +kubebuilder:default="corail:latest"
	Image string `json:"image,omitempty"`

	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	Replicas *int32 `json:"replicas,omitempty"`

	// Suggestions provider: "llm" (dynamic follow-ups) or "static" (from config)
	// +optional
	SuggestionsProvider string `json:"suggestionsProvider,omitempty"`

	// Static suggestions shown on empty chat (JSON array of strings)
	// +optional
	Suggestions string `json:"suggestions,omitempty"`

	// Percentage of production traces to auto-evaluate (0-100, 0=disabled)
	// +optional
	EvalSampleRate int32 `json:"evalSampleRate,omitempty"`

	// Model ID for LLM-judge evaluation (e.g. "openai:/gpt-4o-mini")
	// +optional
	JudgeModel string `json:"judgeModel,omitempty"`

	// Canary deployment configuration
	// +optional
	Canary *CanarySpec `json:"canary,omitempty"`
}

// CanarySpec defines the canary deployment configuration for an Agent.
type CanarySpec struct {
	Enabled      bool     `json:"enabled"`
	Weight       int32    `json:"weight"`
	Image        string   `json:"image,omitempty"`
	ModelType    string   `json:"modelType,omitempty"`
	ModelID      string   `json:"modelId,omitempty"`
	SystemPrompt string   `json:"systemPrompt,omitempty"`
	Skills       []string `json:"skills,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	Version      string   `json:"version,omitempty"`
}

// +kubebuilder:validation:Enum=Pending;Running;Failed;Terminated
type AgentPhase string

const (
	AgentPhasePending    AgentPhase = "Pending"
	AgentPhaseRunning    AgentPhase = "Running"
	AgentPhaseFailed     AgentPhase = "Failed"
	AgentPhaseTerminated AgentPhase = "Terminated"
)

// AgentStatus defines the observed state of Agent
type AgentStatus struct {
	Phase    AgentPhase `json:"phase,omitempty"`
	Replicas int32      `json:"replicas,omitempty"`
	Endpoint string     `json:"endpoint,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.replicas`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec AgentSpec `json:"spec"`

	// +optional
	Status AgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Agent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Agent{}, &AgentList{})
}
