package supportbundle

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	k8sTokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	k8sCACertPath    = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	k8sNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	k8sAPIBase       = "https://kubernetes.default.svc"
)

// Handler manages support bundle Jobs via the Kubernetes API.
type Handler struct {
	image            string
	serviceAccount   string
	imagePullSecrets []string
	sdkEndpoint      string
}

// NewHandler creates a Handler that launches support bundle Jobs using the
// given container image, Kubernetes service account, and optional pull secrets.
// The sdkEndpoint is used to fetch the license ID for uploading bundles.
func NewHandler(image, serviceAccount string, imagePullSecrets []string, sdkEndpoint string) *Handler {
	return &Handler{
		image:            image,
		serviceAccount:   serviceAccount,
		imagePullSecrets: imagePullSecrets,
		sdkEndpoint:      sdkEndpoint,
	}
}

// Generate creates a Kubernetes Job to collect and upload a support bundle,
// returning the job name immediately so the frontend can poll for status.
func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	namespace, err := readFileString(k8sNamespacePath)
	if err != nil {
		log.Printf("support-bundle: failed to read namespace: %v", err)
		writeError(w, "failed to determine namespace", http.StatusInternalServerError)
		return
	}

	client, err := newK8sClient()
	if err != nil {
		log.Printf("support-bundle: failed to create k8s client: %v", err)
		writeError(w, "failed to connect to cluster", http.StatusInternalServerError)
		return
	}

	li := h.fetchLicenseInfo()

	jobName := fmt.Sprintf("support-bundle-%d", time.Now().Unix())

	if err := h.createJob(r.Context(), client, namespace, jobName, li); err != nil {
		log.Printf("support-bundle: failed to create job %s: %v", jobName, err)
		writeError(w, "failed to start support bundle collection", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"name":   jobName,
		"status": "running",
	})
}

// Status returns the current status of a support bundle Job.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	jobName := chi.URLParam(r, "name")
	if jobName == "" {
		writeError(w, "job name is required", http.StatusBadRequest)
		return
	}

	namespace, err := readFileString(k8sNamespacePath)
	if err != nil {
		log.Printf("support-bundle: failed to read namespace: %v", err)
		writeError(w, "failed to determine namespace", http.StatusInternalServerError)
		return
	}

	client, err := newK8sClient()
	if err != nil {
		log.Printf("support-bundle: failed to create k8s client: %v", err)
		writeError(w, "failed to connect to cluster", http.StatusInternalServerError)
		return
	}

	url := fmt.Sprintf("%s/apis/batch/v1/namespaces/%s/jobs/%s", k8sAPIBase, namespace, jobName)
	status, err := getJobStatus(r.Context(), client, url)
	if err != nil {
		log.Printf("support-bundle: failed to get job status for %s: %v", jobName, err)
		writeError(w, "failed to check support bundle status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"name":   jobName,
		"status": status,
	})
}

type licenseInfo struct {
	LicenseID string
	AppSlug   string
}

func (h *Handler) fetchLicenseInfo() licenseInfo {
	resp, err := http.Get(h.sdkEndpoint + "/api/v1/license/info")
	if err != nil {
		log.Printf("support-bundle: failed to fetch license info: %v", err)
		return licenseInfo{}
	}
	defer resp.Body.Close()

	var info struct {
		LicenseID string `json:"licenseID"`
		AppSlug   string `json:"appSlug"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		log.Printf("support-bundle: failed to decode license info: %v", err)
		return licenseInfo{}
	}
	return licenseInfo{LicenseID: info.LicenseID, AppSlug: info.AppSlug}
}

func (h *Handler) buildPodSpec(li licenseInfo) map[string]interface{} {
	cmd := []string{
		"support-bundle",
		"--load-cluster-specs",
		"--auto-upload",
		"--interactive=false",
	}
	if li.LicenseID != "" {
		cmd = append(cmd, fmt.Sprintf("--license-id=%s", li.LicenseID))
	}
	if li.AppSlug != "" {
		cmd = append(cmd, fmt.Sprintf("--app-slug=%s", li.AppSlug))
	}

	spec := map[string]interface{}{
		"serviceAccountName": h.serviceAccount,
		"restartPolicy":      "Never",
		"containers": []map[string]interface{}{
			{
				"name":    "support-bundle",
				"image":   h.image,
				"command": cmd,
			},
		},
	}
	if len(h.imagePullSecrets) > 0 {
		secrets := make([]map[string]string, len(h.imagePullSecrets))
		for i, name := range h.imagePullSecrets {
			secrets[i] = map[string]string{"name": name}
		}
		spec["imagePullSecrets"] = secrets
	}
	return spec
}

func (h *Handler) createJob(ctx context.Context, client *http.Client, namespace, jobName string, li licenseInfo) error {
	ttl := 300
	backoff := int32(0)
	job := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name":      jobName,
			"namespace": namespace,
			"labels": map[string]string{
				"app.kubernetes.io/component":  "support-bundle",
				"app.kubernetes.io/managed-by": "asset-tracker",
			},
		},
		"spec": map[string]interface{}{
			"ttlSecondsAfterFinished": ttl,
			"backoffLimit":            backoff,
			"template": map[string]interface{}{
				"spec": h.buildPodSpec(li),
			},
		},
	}

	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	url := fmt.Sprintf("%s/apis/batch/v1/namespaces/%s/jobs", k8sAPIBase, namespace)
	return doK8sRequest(ctx, client, http.MethodPost, url, body, http.StatusCreated)
}

func getJobStatus(ctx context.Context, client *http.Client, url string) (string, error) {
	token, err := readFileString(k8sTokenPath)
	if err != nil {
		return "", fmt.Errorf("read token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	var job struct {
		Status struct {
			Succeeded int `json:"succeeded"`
			Failed    int `json:"failed"`
		} `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if job.Status.Succeeded > 0 {
		return "succeeded", nil
	}
	if job.Status.Failed > 0 {
		return "failed", nil
	}
	return "running", nil
}

func doK8sRequest(ctx context.Context, client *http.Client, method, url string, body []byte, expectStatus int) error {
	token, err := readFileString(k8sTokenPath)
	if err != nil {
		return fmt.Errorf("read token: %w", err)
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectStatus {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func newK8sClient() (*http.Client, error) {
	caCert, err := os.ReadFile(k8sCACertPath)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caCert)

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: pool,
			},
		},
		Timeout: 30 * time.Second,
	}, nil
}

func readFileString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func writeError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
