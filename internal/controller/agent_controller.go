/*
Copyright 2026 Sciences44.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentsv1 "github.com/sciences44/recif-operator/api/v1"
)

var log = logf.Log.WithName("agent-controller")

// AgentReconciler reconciles Agent objects
type AgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=agents.recif.dev,resources=agents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agents.recif.dev,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agents.recif.dev,resources=agents/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *AgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.WithValues("agent", req.NamespacedName)

	agent := &agentsv1.Agent{}
	if err := r.Get(ctx, req.NamespacedName, agent); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Agent deleted, resources cleaned up via ownerReferences")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("Reconciling agent", "name", agent.Spec.Name, "framework", agent.Spec.Framework)

	for _, ensure := range []func(context.Context, *agentsv1.Agent) error{
		r.ensureConfigMap,
		r.ensureDeployment,
		r.ensureService,
	} {
		if err := ensure(ctx, agent); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update status — re-fetch to avoid conflict
	if err := r.Get(ctx, req.NamespacedName, agent); err != nil {
		return ctrl.Result{}, err
	}
	agent.Status.Phase = agentsv1.AgentPhaseRunning
	agent.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local:8000", agent.Name, agent.Namespace)
	if agent.Spec.Replicas != nil {
		agent.Status.Replicas = *agent.Spec.Replicas
	}
	if err := r.Status().Update(ctx, agent); err != nil {
		logger.Info("Status update conflict, will retry", "error", err)
		return ctrl.Result{Requeue: true}, nil
	}

	logger.Info("Agent reconciled", "endpoint", agent.Status.Endpoint)
	return ctrl.Result{}, nil
}

func (r *AgentReconciler) ensureConfigMap(ctx context.Context, agent *agentsv1.Agent) error {
	name := types.NamespacedName{Name: agent.Name + "-config", Namespace: agent.Namespace}
	desired := r.buildConfigMap(ctx, agent, name)

	existing := &corev1.ConfigMap{}
	if err := r.Get(ctx, name, existing); err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, desired)
		}
		return err
	}
	existing.Data = desired.Data
	return r.Update(ctx, existing)
}

func (r *AgentReconciler) ensureDeployment(ctx context.Context, agent *agentsv1.Agent) error {
	name := types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}
	desired := r.buildDeployment(ctx, agent, name)

	existing := &appsv1.Deployment{}
	if err := r.Get(ctx, name, existing); err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, desired)
		}
		return err
	}
	existing.Spec = desired.Spec
	return r.Update(ctx, existing)
}

func (r *AgentReconciler) ensureService(ctx context.Context, agent *agentsv1.Agent) error {
	name := types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}
	desired := r.buildService(agent, name)

	existing := &corev1.Service{}
	if err := r.Get(ctx, name, existing); err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, desired)
		}
		return err
	}
	return nil // Services don't need spec updates typically
}

// --- Builders (pure functions, no side effects) ---

func (r *AgentReconciler) buildConfigMap(ctx context.Context, agent *agentsv1.Agent, name types.NamespacedName) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name.Name, Namespace: name.Namespace, Labels: labels(agent),
		},
		Data: r.configData(ctx, agent),
	}
	_ = ctrl.SetControllerReference(agent, cm, r.Scheme)
	return cm
}

func (r *AgentReconciler) buildDeployment(ctx context.Context, agent *agentsv1.Agent, name types.NamespacedName) *appsv1.Deployment {
	replicas := int32(1)
	if agent.Spec.Replicas != nil {
		replicas = *agent.Spec.Replicas
	}
	image := agent.Spec.Image
	if image == "" {
		image = "corail:latest"
	}

	// Local images (no registry prefix) must use Never to avoid pulling from Docker Hub
	pullPolicy := corev1.PullIfNotPresent
	if !strings.Contains(image, "/") {
		pullPolicy = corev1.PullNever
	}

	// Hash the config data so pods restart when ConfigMap changes
	configJSON, _ := json.Marshal(r.configData(ctx, agent))
	configHash := fmt.Sprintf("%x", sha256.Sum256(configJSON))

	// Resolve env secrets — default to ["agent-env"] for backward compatibility
	envSecrets := agent.Spec.EnvSecrets
	if len(envSecrets) == 0 {
		envSecrets = []string{"agent-env"}
	}

	// Resolve credential secret — default to "gcp-adc" for backward compatibility
	credentialSecret := agent.Spec.CredentialSecret
	if credentialSecret == "" {
		credentialSecret = "gcp-adc"
	}

	// Build envFrom: always include the ConfigMap, then each secret
	envFrom := []corev1.EnvFromSource{
		{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: agent.Name + "-config"},
			},
		},
	}
	for _, secretName := range envSecrets {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Optional:             ptrBool(true),
			},
		})
	}

	// Build volume mounts and volumes for GCP credentials
	var volumeMounts []corev1.VolumeMount
	var volumes []corev1.Volume
	var envVars []corev1.EnvVar

	if agent.Spec.GCPServiceAccount != "" {
		// Per-agent GCP service account key: secret "{agent}-gcp-sa" with key "credentials.json"
		saSecretName := agent.Name + "-gcp-sa"
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "gcp-sa", MountPath: "/var/secrets/gcp", ReadOnly: true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "gcp-sa",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: saSecretName,
					Optional:   ptrBool(false), // fail loud if SA secret is missing
				},
			},
		})
		envVars = append(envVars, corev1.EnvVar{
			Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secrets/gcp/credentials.json",
		})
		// Extract project ID from SA email: "name@project.iam.gserviceaccount.com"
		if parts := strings.SplitN(agent.Spec.GCPServiceAccount, "@", 2); len(parts) == 2 {
			project := strings.TrimSuffix(parts[1], ".iam.gserviceaccount.com")
			if project != "" {
				envVars = append(envVars, corev1.EnvVar{
					Name: "GOOGLE_CLOUD_PROJECT", Value: project,
				})
			}
		}
	} else {
		// Fallback: shared credential secret (backward compatible)
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name: "credential-secret", MountPath: "/var/secrets/gcp", ReadOnly: true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "credential-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: credentialSecret,
					Optional:   ptrBool(true),
				},
			},
		})
		envVars = append(envVars, corev1.EnvVar{
			Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "/var/secrets/gcp/adc.json",
		})
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name.Name, Namespace: name.Namespace, Labels: labels(agent),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels(agent)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels(agent),
					Annotations: map[string]string{"recif.dev/config-hash": configHash},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "corail", Image: image, ImagePullPolicy: pullPolicy,
						Ports: []corev1.ContainerPort{
						{ContainerPort: 8000, Name: "http"},
						{ContainerPort: 8001, Name: "control"},
					},
						EnvFrom:        envFrom,
						VolumeMounts:   volumeMounts,
						Env:            envVars,
						LivenessProbe:  httpProbe("/healthz", 30, 10),
						ReadinessProbe: httpProbe("/healthz", 15, 5),
					}},
					Volumes: volumes,
				},
			},
		},
	}
	_ = ctrl.SetControllerReference(agent, dep, r.Scheme)
	return dep
}

func (r *AgentReconciler) buildService(agent *agentsv1.Agent, name types.NamespacedName) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name.Name, Namespace: name.Namespace, Labels: labels(agent),
		},
		Spec: corev1.ServiceSpec{
			Selector: labels(agent),
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 8000, TargetPort: intstr.FromInt32(8000), Protocol: corev1.ProtocolTCP},
				{Name: "control", Port: 8001, TargetPort: intstr.FromInt32(8001), Protocol: corev1.ProtocolTCP},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	_ = ctrl.SetControllerReference(agent, svc, r.Scheme)
	return svc
}

// --- Helpers ---

func (r *AgentReconciler) configData(ctx context.Context, agent *agentsv1.Agent) map[string]string {
	data := map[string]string{
		"CORAIL_AGENT_NAME":   agent.Name,
		"RECIF_AGENT_VERSION": fmt.Sprintf("%d", agent.Generation),
		"CORAIL_CHANNEL":       agent.Spec.Channel,
		"CORAIL_STRATEGY":      agent.Spec.Strategy,
		"CORAIL_MODEL_TYPE":    agent.Spec.ModelType,
		"CORAIL_MODEL_ID":      agent.Spec.ModelID,
		"CORAIL_SYSTEM_PROMPT": agent.Spec.SystemPrompt,
	}
	if agent.Spec.Storage != "" {
		data["CORAIL_STORAGE"] = agent.Spec.Storage
	}
	if agent.Spec.DatabaseURL != "" {
		data["CORAIL_DATABASE_URL"] = agent.Spec.DatabaseURL
	}

	// Ollama — pass the base URL with FQDN for cross-namespace resolution
	if ollamaURL := os.Getenv("OLLAMA_BASE_URL"); ollamaURL != "" {
		// If the URL uses a short hostname (no dots), append the operator's namespace
		// so agents in other namespaces can resolve it
		if !strings.Contains(strings.TrimPrefix(strings.TrimPrefix(ollamaURL, "http://"), "https://"), ".") {
			// Extract host:port, make it FQDN
			operatorNS := os.Getenv("POD_NAMESPACE")
			if operatorNS == "" {
				operatorNS = "recif-system"
			}
			ollamaURL = strings.Replace(ollamaURL, "://", "://"+""  , 0)
			// Simple approach: just append .{ns}.svc.cluster.local before the port
			parts := strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(ollamaURL, "http://"), "https://"), ":", 2)
			if len(parts) == 2 {
				ollamaURL = "http://" + parts[0] + "." + operatorNS + ".svc.cluster.local:" + parts[1]
			}
		}
		data["OLLAMA_BASE_URL"] = ollamaURL
	}

	// MLflow — only inject if configured (avoids blocking agent startup when MLflow is not deployed)
	if mlflowURI := os.Getenv("MLFLOW_TRACKING_URI"); mlflowURI != "" {
		data["MLFLOW_TRACKING_URI"] = mlflowURI
	}

	// Resolve Tool CRDs and serialize as JSON for Corail
	if len(agent.Spec.Tools) > 0 {
		toolsJSON := r.resolveTools(ctx, agent)
		if toolsJSON != "" {
			data["CORAIL_TOOLS"] = toolsJSON
		}
	}

	// Resolve Knowledge Bases for RAG strategy
	if len(agent.Spec.KnowledgeBases) > 0 {
		kbJSON := r.resolveKnowledgeBases(agent)
		if kbJSON != "" {
			data["CORAIL_KNOWLEDGE_BASES"] = kbJSON
		}
	}

	// Resolve Skills — serialize as JSON array for Corail
	if len(agent.Spec.Skills) > 0 {
		if skillsJSON, err := json.Marshal(agent.Spec.Skills); err == nil {
			data["CORAIL_SKILLS"] = string(skillsJSON)
		}
	}

	// Eval & suggestions settings (configurable per-agent from dashboard)
	optionalSettings := map[string]string{
		"CORAIL_SUGGESTIONS_PROVIDER": agent.Spec.SuggestionsProvider,
		"CORAIL_SUGGESTIONS":          agent.Spec.Suggestions,
		"RECIF_JUDGE_MODEL":           agent.Spec.JudgeModel,
	}
	for envKey, value := range optionalSettings {
		if value != "" {
			data[envKey] = value
		}
	}
	if agent.Spec.EvalSampleRate > 0 {
		data["RECIF_EVAL_SAMPLE_RATE"] = fmt.Sprintf("0.%02d", agent.Spec.EvalSampleRate)
	}

	// Vertex AI / Google Cloud — inject project + location
	if agent.Spec.ModelType == "vertex-ai" || agent.Spec.ModelType == "google-ai" {
		// Priority: GCPServiceAccount email > operator env var
		if agent.Spec.GCPServiceAccount != "" {
			if parts := strings.SplitN(agent.Spec.GCPServiceAccount, "@", 2); len(parts) == 2 {
				project := strings.TrimSuffix(parts[1], ".iam.gserviceaccount.com")
				if project != "" {
					data["GOOGLE_CLOUD_PROJECT"] = project
				}
			}
		} else if project := os.Getenv("GOOGLE_CLOUD_PROJECT"); project != "" {
			data["GOOGLE_CLOUD_PROJECT"] = project
		}
		if location := os.Getenv("GOOGLE_CLOUD_LOCATION"); location != "" {
			data["GOOGLE_CLOUD_LOCATION"] = location
		}
	}

	return data
}

// resolveTools reads Tool CRDs by name and returns a JSON array for Corail.
func (r *AgentReconciler) resolveTools(ctx context.Context, agent *agentsv1.Agent) string {
	type toolConfig struct {
		Name            string            `json:"name"`
		Type            string            `json:"type"`
		Description     string            `json:"description,omitempty"`
		Endpoint        string            `json:"endpoint,omitempty"`
		Method          string            `json:"method,omitempty"`
		Headers         map[string]string `json:"headers,omitempty"`
		Binary          string            `json:"binary,omitempty"`
		AllowedCommands []string          `json:"allowedCommands,omitempty"`
		MCPEndpoint     string            `json:"mcpEndpoint,omitempty"`
		SecretRef       string            `json:"secretRef,omitempty"`
		Timeout         int32             `json:"timeout,omitempty"`
	}

	var tools []toolConfig
	for _, toolName := range agent.Spec.Tools {
		tool := &agentsv1.Tool{}
		if err := r.Get(ctx, types.NamespacedName{Name: toolName, Namespace: agent.Namespace}, tool); err != nil {
			// No CRD found — treat as a builtin tool (e.g. web-search, datetime).
			// Warn so typos are visible in operator logs.
			log.Info("WARN: Tool CRD not found, treating as builtin — verify name is correct", "tool", toolName, "namespace", agent.Namespace)
			tools = append(tools, toolConfig{Name: toolName, Type: "builtin"})
			continue
		}
		if !tool.Spec.Enabled {
			continue
		}
		tools = append(tools, toolConfig{
			Name:            tool.Spec.Name,
			Type:            tool.Spec.Type,
			Description:     tool.Spec.Description,
			Endpoint:        tool.Spec.Endpoint,
			Method:          tool.Spec.Method,
			Headers:         tool.Spec.Headers,
			Binary:          tool.Spec.Binary,
			AllowedCommands: tool.Spec.AllowedCommands,
			MCPEndpoint:     tool.Spec.MCPEndpoint,
			SecretRef:       tool.Spec.SecretRef,
			Timeout:         tool.Spec.Timeout,
		})
	}

	if len(tools) == 0 {
		return ""
	}

	data, err := json.Marshal(tools)
	if err != nil {
		log.Error(err, "Failed to marshal tools config")
		return ""
	}
	return string(data)
}

// resolveKnowledgeBases builds the CORAIL_KNOWLEDGE_BASES JSON from CRD spec.
func (r *AgentReconciler) resolveKnowledgeBases(agent *agentsv1.Agent) string {
	type kbConfig struct {
		Name              string `json:"name"`
		Type              string `json:"type"`
		ConnectionURL     string `json:"connection_url"`
		KBID              string `json:"kb_id"`
		EmbeddingProvider string `json:"embedding_provider"`
		EmbeddingModel    string `json:"embedding_model"`
	}

	// For now, all KBs use pgvector in the same PostgreSQL + Ollama embeddings.
	// The connection URL points to the cluster-internal PostgreSQL.
	var kbs []kbConfig
	for _, kbName := range agent.Spec.KnowledgeBases {
		kbs = append(kbs, kbConfig{
			Name:              kbName,
			Type:              "pgvector",
			ConnectionURL:     "postgresql://recif:recif@recif-postgresql.recif-system.svc.cluster.local:5432/corail_storage",
			KBID:              kbName,
			EmbeddingProvider: "ollama",
			EmbeddingModel:    "nomic-embed-text",
		})
	}

	if len(kbs) == 0 {
		return ""
	}

	data, err := json.Marshal(kbs)
	if err != nil {
		log.Error(err, "Failed to marshal KB config")
		return ""
	}
	return string(data)
}

func labels(agent *agentsv1.Agent) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       agent.Name,
		"app.kubernetes.io/part-of":    "recif",
		"app.kubernetes.io/managed-by": "recif-operator",
		"recif.dev/agent":              agent.Name,
	}
}

func ptrBool(b bool) *bool { return &b }

func httpProbe(path string, initialDelay, period int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: path, Port: intstr.FromInt32(8000)}},
		InitialDelaySeconds: initialDelay,
		PeriodSeconds:       period,
	}
}

func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentsv1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
