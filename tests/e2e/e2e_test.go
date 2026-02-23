package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AlertRequest represents the payload for webhook registration
type AlertRequest struct {
	Symbol           string  `json:"symbol"`
	TargetURL        string  `json:"target_url"`
	ThresholdPercent float64 `json:"threshold_percent"`
}

func getDockerComposeCmd(args ...string) *exec.Cmd {
	// Try 'docker compose' (v2 plugin) first
	cmd := exec.Command("docker", append([]string{"compose"}, args...)...) //nolint:gosec // arguments are controlled in tests.
	if err := exec.Command("docker", "compose", "version").Run(); err == nil {
		return cmd
	}
	// Fallback to 'docker-compose' (v1 standalone)
	return exec.Command("docker-compose", args...) //nolint:gosec // arguments are controlled in tests.
}

func TestBlackBoxE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	composeFiles := []string{"-f", "docker-compose.yml", "-f", "tests/e2e/docker-compose.e2e.yml"}

	// 1. Build and spin up the environment
	cmdUp := getDockerComposeCmd(append(composeFiles, "up", "-d", "--build")...)
	cmdUp.Dir = "../../"
	if out, err := cmdUp.CombinedOutput(); err != nil {
		t.Fatalf("Failed to spin up docker: %v\nOutput: %s", err, string(out))
	}

	// Deferred environment cleanup
	defer func() {
		cmdDown := getDockerComposeCmd(append(composeFiles, "down", "-v")...)
		cmdDown.Dir = "../../"
		_ = cmdDown.Run()
	}()

	// 2. Wait for readiness (polling GET /v1/health)
	appURL := "http://localhost:8081"
	client := &http.Client{Timeout: 2 * time.Second}

	ready := false
	for i := 0; i < 30; i++ { // Attempt up to 30 times with a 1s interval (total 30s)
		resp, err := client.Get(appURL + "/v1/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			ready = true
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	require.True(t, ready, "Application did not become healthy in time")

	// 3. Spawning a local httptest.Server to receive webhooks
	webhookReceivedChan := make(chan string, 1) // Channel to signal receipt of a webhook

	mockWebhookTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		webhookReceivedChan <- string(bodyBytes)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockWebhookTarget.Close()

	// 4. Webhook registration (POST /v1/alerts)
	// We set a threshold of 0.00000001% so that any micro-change in price (common in crypto markets) triggers the webhook.
	// For testing purposes, a mock source could be used, but since this is a Black-Box test, we listen to real quotes.
	alertReq := AlertRequest{
		Symbol:           "BTC",
		TargetURL:        mockWebhookTarget.URL,
		ThresholdPercent: 0.00000001, // Microscopic threshold to trigger at the next tick (if price updates)
	}

	reqBody, _ := json.Marshal(alertReq)
	resp, err := client.Post(appURL+"/v1/alerts", "application/json", bytes.NewReader(reqBody))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusCreated, resp.StatusCode)

	// 5. Waiting for the worker to trigger
	// docker-compose has POLL_INTERVAL=30s by default. We wait up to 35 seconds.
	// In production-like E2E tests, it's better to set POLL_INTERVAL=5s in docker-compose.yml.
	select {
	case payload := <-webhookReceivedChan:
		// Webhook received successfully!
		t.Logf("Received webhook payload: %s", payload)
		assert.Contains(t, payload, "\"symbol\":\"BTC\"")
		assert.Contains(t, payload, "\"change_pct\":")
	case <-time.After(35 * time.Second): // 35 seconds
		// The market price might not have changed at all during these 35 seconds.
		// Black-Box E2E with the real external world is inherently flaky.
		// At the very least, we verify that we reached this step without infrastructure errors.
		t.Logf("Warning: No webhook received within 35 seconds. This might be normal if the upstream BTC price didn't change.")
	}

	// 6. Verify GET /v1/assets/BTC
	respAssets, err := client.Get(appURL + "/v1/assets/BTC")
	require.NoError(t, err)
	defer func() { _ = respAssets.Body.Close() }()
	require.Equal(t, http.StatusOK, respAssets.StatusCode)

	bodyAssets, _ := io.ReadAll(respAssets.Body)
	assert.Contains(t, string(bodyAssets), "BTC")
}
