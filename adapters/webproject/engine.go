// Package webproject serves a local web experience and runs it in Chromium as
// one Jangolova engine instance.
package webproject

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"jangolova/adapters/chromium"
	"jangolova/internal/engineprovider"
	"jangolova/internal/manifest"
	"jangolova/internal/orchestrator"
)

type Adapter struct{}

type mount struct {
	URLPath string `json:"urlPath"`
	Source  string `json:"source"`
}

type options struct {
	Entry    string          `json:"entry,omitempty"`
	Mounts   []mount         `json:"mounts,omitempty"`
	Chromium json.RawMessage `json:"chromium,omitempty"`
}

type instance struct {
	mu       sync.Mutex
	browser  orchestrator.EngineInstance
	endpoint string
	events   <-chan orchestrator.EngineEvent
	server   *http.Server
}

type cdpEndpointProvider interface {
	CDPEndpoint() string
}

var _ orchestrator.EngineAdapter = Adapter{}
var _ orchestrator.EngineInstance = (*instance)(nil)
var _ orchestrator.EngineEventSource = (*instance)(nil)
var _ orchestrator.EngineHealthProvider = (*instance)(nil)
var _ orchestrator.EngineInspector = Adapter{}
var _ engineprovider.EndpointProvider = (*instance)(nil)

func (Adapter) InspectEngine(ctx context.Context) orchestrator.EngineInspection {
	browser := (chromium.Adapter{}).InspectEngine(ctx)
	launchAvailable := false
	for _, capability := range browser.Capabilities {
		if capability == "launch" {
			launchAvailable = true
			break
		}
	}
	inspection := orchestrator.EngineInspection{
		Available: launchAvailable,
		Capabilities: []string{
			"endpoint.cdp",
			"events",
			"health",
			"runtime.environment",
			"stop",
		},
	}
	if launchAvailable {
		inspection.Capabilities = append(inspection.Capabilities, "launch")
	} else {
		inspection.Message = "web-project requires Chromium launch support: " + browser.Message
	}
	return inspection
}

func (Adapter) Start(
	ctx context.Context,
	spec manifest.EngineSpec,
	launch orchestrator.EngineRuntime,
) (orchestrator.EngineInstance, error) {
	config, err := decodeOptions(spec.Options)
	if err != nil {
		return nil, err
	}
	root := strings.TrimSpace(spec.Source)
	if root == "" {
		return nil, errors.New("web-project engine source directory is required")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve web-project source: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("open web-project source: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("web-project source %q is not a directory", root)
	}
	entry := strings.TrimLeft(strings.TrimSpace(config.Entry), "/")
	if entry == "" {
		entry = "index.html"
	}
	entry, err = cleanRelativePath(entry)
	if err != nil {
		return nil, fmt.Errorf("web-project entry %q: %w", config.Entry, err)
	}
	if _, err := os.Stat(filepath.Join(root, entry)); err != nil {
		return nil, fmt.Errorf("open web-project entry %q: %w", entry, err)
	}

	mux := http.NewServeMux()
	registeredPaths := make(map[string]struct{}, len(config.Mounts))
	for _, configuredMount := range config.Mounts {
		urlPath := strings.TrimSpace(configuredMount.URLPath)
		source := strings.TrimSpace(configuredMount.Source)
		parsedPath, parseErr := url.ParseRequestURI(urlPath)
		if parseErr != nil ||
			parsedPath.Path != urlPath ||
			!strings.HasPrefix(urlPath, "/") ||
			strings.HasSuffix(urlPath, "/") ||
			strings.Contains(urlPath, "..") {
			return nil, fmt.Errorf(
				"web-project mount urlPath %q must be an absolute, clean file path",
				urlPath,
			)
		}
		if _, exists := registeredPaths[urlPath]; exists {
			return nil, fmt.Errorf("web-project mount urlPath %q is duplicated", urlPath)
		}
		registeredPaths[urlPath] = struct{}{}
		if source == "" {
			return nil, fmt.Errorf("web-project mount %q source is required", urlPath)
		}
		source, err = filepath.Abs(source)
		if err != nil {
			return nil, fmt.Errorf("resolve web-project mount %q: %w", urlPath, err)
		}
		if info, err := os.Stat(source); err != nil || info.IsDir() {
			return nil, fmt.Errorf("web-project mount %q source must be a file", urlPath)
		}
		mux.HandleFunc(urlPath, func(writer http.ResponseWriter, request *http.Request) {
			http.ServeFile(writer, request, source)
		})
	}
	mux.Handle("/", http.FileServer(http.Dir(root)))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for web project: %w", err)
	}
	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()

	pageURL := (&url.URL{
		Scheme: "http",
		Host:   listener.Addr().String(),
		Path:   "/" + filepath.ToSlash(entry),
	}).String()
	browser, err := (chromium.Adapter{}).Start(ctx, manifest.EngineSpec{
		Adapter: "chromium",
		Source:  pageURL,
		Options: config.Chromium,
	}, launch)
	if err != nil {
		_ = server.Shutdown(context.Background())
		return nil, fmt.Errorf("start web-project browser: %w", err)
	}
	endpointProvider, ok := browser.(cdpEndpointProvider)
	if !ok {
		_ = browser.Stop(context.Background())
		_ = server.Shutdown(context.Background())
		return nil, errors.New("web-project Chromium instance did not expose CDP")
	}
	events := make(chan orchestrator.EngineEvent, 1)
	if source, ok := browser.(orchestrator.EngineEventSource); ok {
		go func() {
			for event := range source.EngineEvents() {
				events <- event
				if event.Status == "exited" || event.Status == "failed" {
					_ = server.Shutdown(context.Background())
				}
			}
			close(events)
		}()
	} else {
		close(events)
	}
	return &instance{
		browser:  browser,
		endpoint: endpointProvider.CDPEndpoint(),
		events:   events,
		server:   server,
	}, nil
}

func decodeOptions(raw json.RawMessage) (options, error) {
	if len(raw) == 0 {
		return options{}, nil
	}
	var value options
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return options{}, fmt.Errorf("decode web-project engine options: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return options{}, errors.New("decode web-project engine options: multiple JSON values")
	}
	return value, nil
}

func cleanRelativePath(value string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(value))
	if cleaned == "." ||
		filepath.IsAbs(cleaned) ||
		cleaned == ".." ||
		strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("must identify a file inside the source directory")
	}
	return cleaned, nil
}

func (i *instance) CDPEndpoint() string {
	return i.endpoint
}

func (i *instance) EngineEndpoints() []engineprovider.Endpoint {
	parsed, _ := url.Parse(i.endpoint)
	port, _ := strconv.Atoi(parsed.Port())
	return []engineprovider.Endpoint{{
		Name:       "cdp",
		Protocol:   "cdp",
		URL:        i.endpoint,
		TargetPort: port,
		Visibility: "private",
	}}
}

func (i *instance) EngineEvents() <-chan orchestrator.EngineEvent {
	return i.events
}

func (i *instance) EngineHealth(ctx context.Context) orchestrator.EngineHealth {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.browser == nil {
		return orchestrator.EngineHealth{
			Status:     orchestrator.EngineHealthStopped,
			ObservedAt: time.Now().UTC(),
		}
	}
	if provider, ok := i.browser.(orchestrator.EngineHealthProvider); ok {
		return provider.EngineHealth(ctx)
	}
	return orchestrator.EngineHealth{
		Status:     orchestrator.EngineHealthUnknown,
		Message:    "browser engine does not implement an active health probe",
		ObservedAt: time.Now().UTC(),
	}
}

func (i *instance) Stop(ctx context.Context) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	var problems []error
	if i.browser != nil {
		if err := i.browser.Stop(ctx); err != nil {
			problems = append(problems, fmt.Errorf("stop web-project browser: %w", err))
		}
		i.browser = nil
	}
	if i.server != nil {
		if err := i.server.Shutdown(ctx); err != nil {
			problems = append(problems, fmt.Errorf("stop web-project server: %w", err))
		}
		i.server = nil
	}
	return errors.Join(problems...)
}
