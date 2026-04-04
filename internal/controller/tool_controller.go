/*
Copyright 2026 Sciences44.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentsv1 "github.com/sciences44/recif-operator/api/v1"
)

var toolLog = logf.Log.WithName("tool-controller")

// ToolReconciler reconciles Tool objects
type ToolReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agents.recif.dev,resources=tools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agents.recif.dev,resources=tools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agents.recif.dev,resources=tools/finalizers,verbs=update

func (r *ToolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := toolLog.WithValues("tool", req.NamespacedName)

	tool := &agentsv1.Tool{}
	if err := r.Get(ctx, req.NamespacedName, tool); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Tool deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling tool", "name", tool.Spec.Name, "type", tool.Spec.Type)

	// Validate the tool spec
	if errMsg := validateTool(tool); errMsg != "" {
		return r.setStatus(ctx, req, tool, agentsv1.ToolPhaseError, errMsg)
	}

	return r.setStatus(ctx, req, tool, agentsv1.ToolPhaseAvailable, "")
}

func (r *ToolReconciler) setStatus(ctx context.Context, req ctrl.Request, tool *agentsv1.Tool, phase agentsv1.ToolPhase, message string) (ctrl.Result, error) {
	logger := toolLog.WithValues("tool", req.NamespacedName)

	// Re-fetch to avoid conflict
	if err := r.Get(ctx, req.NamespacedName, tool); err != nil {
		return ctrl.Result{}, err
	}

	tool.Status.Phase = phase
	tool.Status.Message = message

	if err := r.Status().Update(ctx, tool); err != nil {
		logger.Info("Status update conflict, will retry", "error", err)
		return ctrl.Result{Requeue: true}, nil
	}

	logger.Info("Tool reconciled", "phase", phase)
	return ctrl.Result{}, nil
}

// validateTool checks that the tool spec is valid for its type.
// Returns an empty string if valid, or an error message if not.
func validateTool(tool *agentsv1.Tool) string {
	validators := map[string]func(*agentsv1.ToolSpec) string{
		"http":    validateHTTPTool,
		"cli":     validateCLITool,
		"mcp":     validateMCPTool,
		"builtin": validateBuiltinTool,
	}

	validate, ok := validators[tool.Spec.Type]
	if !ok {
		return fmt.Sprintf("unknown tool type: %s", tool.Spec.Type)
	}
	return validate(&tool.Spec)
}

func validateHTTPTool(spec *agentsv1.ToolSpec) string {
	if spec.Endpoint == "" {
		return "HTTP tool requires an endpoint"
	}
	return ""
}

func validateCLITool(spec *agentsv1.ToolSpec) string {
	if spec.Binary == "" {
		return "CLI tool requires a binary"
	}
	return ""
}

func validateMCPTool(spec *agentsv1.ToolSpec) string {
	if spec.MCPEndpoint == "" {
		return "MCP tool requires an mcpEndpoint"
	}
	return ""
}

func validateBuiltinTool(_ *agentsv1.ToolSpec) string {
	return ""
}

func (r *ToolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentsv1.Tool{}).
		Complete(r)
}
