package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupExecutor(tb testing.TB) *DockerExecutor {
	tb.Helper()

	exec, err := NewDockerExecutor()
	if err != nil {
		tb.Skipf("Skipping test: Docker daemon unavailable: %v", err)
	}
	return exec
}

func TestDockerExecutor_Execute_Success(t *testing.T) {
	exec := setupExecutor(t)
	ctx := context.Background()

	job := Job{
		ID:       1,
		Name:     "test-build",
		Image:    "alpine:latest",
		Commands: []string{"echo 'Hello Pipe CI'", "echo 'Step 2 completed'"},
		WorkDir:  "/workspace",
	}

	res, err := exec.Execute(ctx, job)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !res.Success {
		t.Errorf("expected job success, got failure with exit code: %d", res.ExitCode)
	}

	if res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", res.ExitCode)
	}

	if !strings.Contains(res.Logs, "Hello Pipe CI") {
		t.Errorf("expected logs to contain 'Hello Pipe CI', got:\n%s", res.Logs)
	}

	if !strings.Contains(res.Logs, "Step 2 completed") {
		t.Errorf("expected logs to contain 'Step 2 completed', got:\n%s", res.Logs)
	}
}

func TestDockerExecutor_Execute_CommandFailure(t *testing.T) {
	exec := setupExecutor(t)
	ctx := context.Background()

	job := Job{
		ID:       2,
		Name:     "failing-job",
		Image:    "alpine:latest",
		Commands: []string{"echo 'starting execution'", "exit 42"},
	}

	res, err := exec.Execute(ctx, job)
	if err != nil {
		t.Fatalf("expected execution to return result, got error: %v", err)
	}

	if res.Success {
		t.Errorf("expected job failure, got success")
	}

	if res.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", res.ExitCode)
	}
}

func TestDockerExecutor_Execute_EnvironmentVariables(t *testing.T) {
	exec := setupExecutor(t)
	ctx := context.Background()

	job := Job{
		ID:       3,
		Name:     "env-check",
		Image:    "alpine:latest",
		Env:      []string{"FOO=bar", "BUILD_ID=101"},
		Commands: []string{"echo FOO=$FOO BUILD_ID=$BUILD_ID"},
	}

	res, err := exec.Execute(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(res.Logs, "FOO=bar BUILD_ID=101") {
		t.Errorf("expected environment variables to be populated, got logs:\n%s", res.Logs)
	}
}

func TestDockerExecutor_Execute_VolumeMount(t *testing.T) {
	exec := setupExecutor(t)
	ctx := context.Background()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("pipe-ci-mount-test"), 0644); err != nil {
		t.Fatalf("failed to write host file: %v", err)
	}

	job := Job{
		ID:       4,
		Name:     "mount-check",
		Image:    "alpine:latest",
		MountDir: tempDir,
		WorkDir:  "/workspace",
		Commands: []string{"cat /workspace/test.txt"},
	}

	res, err := exec.Execute(ctx, job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(res.Logs, "pipe-ci-mount-test") {
		t.Errorf("expected mounted file contents in logs, got:\n%s", res.Logs)
	}
}

func TestDockerExecutor_Execute_TimeoutContext(t *testing.T) {
	exec := setupExecutor(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	job := Job{
		ID:       5,
		Name:     "timeout-job",
		Image:    "alpine:latest",
		Commands: []string{"sleep 10"},
	}

	_, err := exec.Execute(ctx, job)
	if err == nil {
		t.Fatal("expected error due to context timeout, got nil")
	}
}

func BenchmarkExecutor_ShortTask(b *testing.B) {
	exec := setupExecutor(b)
	ctx := context.Background()

	job := Job{
		ID:       100,
		Name:     "bench-short",
		Image:    "alpine:latest",
		Commands: []string{"true"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := exec.Execute(ctx, job)
		if err != nil {
			b.Fatalf("benchmark iteration failed: %v", err)
		}
		if !res.Success {
			b.Fatalf("expected job to succeed")
		}
	}
}

func BenchmarkExecutor_HeavyLogs(b *testing.B) {
	exec := setupExecutor(b)
	ctx := context.Background()

	job := Job{
		ID:       101,
		Name:     "bench-logs",
		Image:    "alpine:latest",
		Commands: []string{"for i in $(seq 1 500); do echo \"Log stream line number $i\"; done"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := exec.Execute(ctx, job)
		if err != nil {
			b.Fatalf("benchmark iteration failed: %v", err)
		}
	}
}
