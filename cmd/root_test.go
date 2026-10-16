package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeHTTPSavesRequest(t *testing.T) {
	dir := chdirTemp(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/hook", strings.NewReader(`{"event":"created"}`))
	handler{}.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("want status %d, got %d", http.StatusOK, res.Code)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.http"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 saved request, got %d", len(files))
	}

	saved, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "POST /hook\n") {
		t.Errorf("want saved request to contain the request line, got:\n%s", saved)
	}
	if !strings.HasSuffix(string(saved), `{"event":"created"}`) {
		t.Errorf("want saved request to end with the body, got:\n%s", saved)
	}
}

func TestServeHTTPRunsExecCommandWithBody(t *testing.T) {
	dir := chdirTemp(t)
	h := execHandler(t, "cat > exec-output")

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"event":"created"}`))
	h.ServeHTTP(res, req)

	want := `{"event":"created"}`
	got := waitForFile(t, filepath.Join(dir, "exec-output"), len(want))
	if got != want {
		t.Errorf("want exec command stdin %q, got %q", want, got)
	}
}

func TestServeHTTPRunsExecCommandsOneAtATime(t *testing.T) {
	dir := chdirTemp(t)
	h := execHandler(t, "echo start >> exec-log; sleep 0.2; echo end >> exec-log")

	for range 2 {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}")))
	}

	want := "start\nend\nstart\nend\n"
	got := waitForFile(t, filepath.Join(dir, "exec-log"), len(want))
	if got != want {
		t.Errorf("want exec log %q, got %q", want, got)
	}
}

func TestRunExecCommandReportsFailure(t *testing.T) {
	stderr := captureStderr(t)

	runExecCommand("exit 3", nil)

	want := `exec "exit 3": exit status 3`
	if got := stderr(); !strings.Contains(got, want) {
		t.Errorf("want stderr to contain %q, got %q", want, got)
	}
}

func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Error(err)
		}
	})
	return dir
}

func execHandler(t *testing.T, command string) handler {
	t.Helper()
	queue := make(chan []byte, 10)
	go runExecQueue(command, queue)
	t.Cleanup(func() { close(queue) })
	return handler{execQueue: queue}
}

func waitForFile(t *testing.T, path string, size int) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(path)
		if err == nil && len(contents) >= size {
			return string(contents)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
	return ""
}

func captureStderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = previous })

	return func() string {
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		os.Stderr = previous
		out, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(out)
	}
}
