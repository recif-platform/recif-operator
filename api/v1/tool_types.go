/*
Copyright 2026 Sciences44.
Licensed under the Apache License, Version 2.0.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ToolSpec defines the desired state of a Tool
type ToolSpec struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=http;cli;mcp;builtin
	Type string `json:"type"`

	// +kubebuilder:default=general
	Category string `json:"category,omitempty"`

	Description string `json:"description,omitempty"`

	// HTTP tool fields
	Endpoint string `json:"endpoint,omitempty"`
	Method   string `json:"method,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`

	// CLI tool fields
	Binary          string   `json:"binary,omitempty"`
	AllowedCommands []string `json:"allowedCommands,omitempty"`

	// MCP tool fields
	MCPEndpoint string `json:"mcpEndpoint,omitempty"`

	// Common
	SecretRef  string          `json:"secretRef,omitempty"`
	Parameters []ToolParameter `json:"parameters,omitempty"`

	// +kubebuilder:default=30
	Timeout int32 `json:"timeout,omitempty"`

	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`
}

// ToolParameter defines a parameter accepted by a tool
type ToolParameter struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=string;integer;number;boolean
	Type string `json:"type"`

	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
}

// +kubebuilder:validation:Enum=Available;Error
type ToolPhase string

const (
	ToolPhaseAvailable ToolPhase = "Available"
	ToolPhaseError     ToolPhase = "Error"
)

// ToolStatus defines the observed state of Tool
type ToolStatus struct {
	Phase   ToolPhase `json:"phase,omitempty"`
	Message string    `json:"message,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Tool is the Schema for the tools API
type Tool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec ToolSpec `json:"spec"`

	// +optional
	Status ToolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ToolList contains a list of Tool
type ToolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Tool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tool{}, &ToolList{})
}
