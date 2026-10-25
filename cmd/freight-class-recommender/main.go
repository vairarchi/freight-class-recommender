// Command freight-class-recommender exposes the freight class recommender over
// HTTP following the PB appserver template. For the POC demo the routes are
// registered as unauthenticated to bypass token verification.
package main

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/vairarchi/freight-class-recommender/internal/freightclass"
	"github.com/vairarchi/freight-class-recommender/internal/web"
	"gitlab.com/pitneybowes/innovation/pbgolib/appserver"
	pblog "gitlab.com/pitneybowes/innovation/pbgolib/core/logging"
)

// loadEnvFile loads KEY=VALUE lines from path into the environment (without
// overriding already-set vars), so `go run` picks up local config like the
// VS Code envFile does. Missing file is not an error.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k != "" && os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

// setDemoDefaults seeds the env vars the PB appserver requires so the demo runs
// with no external configuration. Real deployments override them.
func setDemoDefaults() {
	defaults := map[string]string{
		"APP_SERVER_PORT":          "8080",
		"APP_SERVER_READ_TIMEOUT":  "15",
		"APP_SERVER_WRITE_TIMEOUT": "15",
		"APP_SERVER_IDLE_TIMEOUT":  "60",
	}
	for k, v := range defaults {
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v)
		}
	}
}

// serveUI returns the embedded single-page demo UI.
func serveUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(web.IndexHTML)
}

func main() {
	loadEnvFile(".vscode/dev.env")
	setDemoDefaults()
	pblog.Debugf(context.TODO(), "freight-class-recommender: starting")

	// AI seam: use the Bedrock LLM matcher when configured, else keyword fallback.
	freightclass.Matcher = freightclass.NewLLMMatcher()

	fcCtx := freightclass.New()

	// Unauthenticated routes (V1) so the demo bypasses token verification.
	routes := []appserver.Route{
		*appserver.NewRoute("/", serveUI, "GET"),
		*appserver.NewRoute("/api/v1/freightclass/recommend", fcCtx.RecommendAPI, "POST"),
	}

	if err := appserver.Start(nil, routes); err != nil {
		pblog.WithError(context.Background(), "Error while starting webserver", err)
		return
	}
}
