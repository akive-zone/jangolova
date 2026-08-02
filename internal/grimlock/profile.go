// Package grimlock implements Jangolova's model-powered agent subsystem.
// Model profiles and protocol adapters belong here; interaction targets remain
// owned by the core engine/provider packages.
package grimlock

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

const ModelAPIVersion = "agent.model/v1alpha1"

var (
	profileIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,127}$`)
	protocolPattern  = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,63}$`)
)

// ModelProfile identifies a caller-selected model gateway. CredentialRef and
// TLSRef are opaque references; resolved material is never serialized here.
type ModelProfile struct {
	APIVersion    string            `json:"apiVersion"`
	ProfileID     string            `json:"profileId"`
	Protocol      string            `json:"protocol"`
	Endpoint      string            `json:"endpoint"`
	Model         string            `json:"model"`
	CredentialRef string            `json:"credentialRef"`
	TLSRef        string            `json:"tlsRef,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

func (p ModelProfile) Validate() error {
	if p.APIVersion != ModelAPIVersion {
		return fmt.Errorf("model apiVersion must be %q", ModelAPIVersion)
	}
	if !profileIDPattern.MatchString(p.ProfileID) {
		return errors.New("model profileId is invalid")
	}
	if !protocolPattern.MatchString(p.Protocol) {
		return errors.New("model protocol is invalid")
	}
	if strings.TrimSpace(p.Model) == "" || len(p.Model) > 256 || strings.ContainsRune(p.Model, '\x00') {
		return errors.New("model name is required")
	}
	if !profileIDPattern.MatchString(p.CredentialRef) {
		return errors.New("model credentialRef is required and must be an opaque reference")
	}
	if p.TLSRef != "" && !profileIDPattern.MatchString(p.TLSRef) {
		return errors.New("model tlsRef must be an opaque reference")
	}
	parsed, err := url.Parse(p.Endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || len(p.Endpoint) > 4096 {
		return errors.New("model endpoint must be an absolute HTTP(S) URL without user information, queries, or fragments")
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		if !isLoopbackHost(parsed.Hostname()) {
			return errors.New("non-loopback model endpoints must use https")
		}
	default:
		return errors.New("model endpoint must use http or https")
	}
	if len(p.Metadata) > 32 {
		return errors.New("model metadata must not exceed 32 entries")
	}
	for name, value := range p.Metadata {
		if !profileIDPattern.MatchString(name) || len(value) > 1024 || strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("model metadata entry %q is invalid", name)
		}
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func cloneModelProfile(profile ModelProfile) ModelProfile {
	result := profile
	result.Metadata = make(map[string]string, len(profile.Metadata))
	for name, value := range profile.Metadata {
		result.Metadata[name] = value
	}
	return result
}
