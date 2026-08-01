package targetconn

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"jangolova/internal/orchestrator"
)

const minimumCredentialValidity = 5 * time.Second

// Prepare resolves all references into a cloned target. Release must be called
// after the adapter disconnects, or immediately if connection fails.
func Prepare(ctx context.Context, resolver Resolver, target orchestrator.EngineTarget) (orchestrator.EngineTarget, func(context.Context) error, error) {
	prepared := cloneTarget(target)
	var materials []*orchestrator.EndpointConnection
	var releases []func(context.Context) error
	var releaseOnce sync.Once
	var releaseErr error
	release := func(releaseCtx context.Context) error {
		releaseOnce.Do(func() {
			var problems []error
			for index := len(releases) - 1; index >= 0; index-- {
				if releases[index] != nil {
					if err := releases[index](releaseCtx); err != nil {
						problems = append(problems, errors.New("release connection material"))
					}
				}
			}
			for _, material := range materials {
				wipeHeaders(material.Headers)
				material.TLS = nil
				material.ExpiresAt = time.Time{}
			}
			releaseErr = errors.Join(problems...)
		})
		return releaseErr
	}
	for index := range prepared.Endpoints {
		endpoint := &prepared.Endpoints[index]
		if endpoint.CredentialRef == "" && endpoint.TLSRef == "" {
			continue
		}
		if resolver == nil {
			_ = release(context.Background())
			return orchestrator.EngineTarget{}, func(context.Context) error { return nil }, errors.New("target connection material resolver is not configured")
		}
		connection := &orchestrator.EndpointConnection{}
		for _, item := range []struct {
			kind      ReferenceKind
			reference string
		}{{CredentialReference, endpoint.CredentialRef}, {TLSReference, endpoint.TLSRef}} {
			if item.reference == "" {
				continue
			}
			material, err := resolver.Resolve(ctx, Request{
				Reference: item.reference, Kind: item.kind, TargetID: target.TargetID,
				EndpointName: endpoint.Name, Protocol: endpoint.Protocol, Audience: endpoint.Audience,
			})
			if err != nil {
				_ = release(context.Background())
				return orchestrator.EngineTarget{}, func(context.Context) error { return nil }, referenceError(item.kind, item.reference, err)
			}
			if err := validateResolvedMaterial(item.kind, material); err != nil {
				if material.Release != nil {
					_ = material.Release(context.Background())
				}
				_ = release(context.Background())
				return orchestrator.EngineTarget{}, func(context.Context) error { return nil }, referenceError(item.kind, item.reference, err)
			}
			if item.kind == CredentialReference {
				connection.Headers = cloneHeaders(material.Headers)
			} else {
				connection.TLS = material.TLS
			}
			connection.ExpiresAt = earliest(connection.ExpiresAt, material.ExpiresAt)
			releases = append(releases, material.Release)
		}
		endpoint.Connection = connection
		materials = append(materials, connection)
	}
	return prepared, release, nil
}

func validateResolvedMaterial(kind ReferenceKind, material Material) error {
	switch kind {
	case CredentialReference:
		if len(material.Headers) == 0 || material.TLS != nil {
			return errors.New("credential material must contain headers only")
		}
		if err := validateHeaders(material.Headers); err != nil {
			return err
		}
		if material.ExpiresAt.IsZero() || !material.ExpiresAt.After(time.Now().Add(minimumCredentialValidity)) {
			return errors.New("connection material is expired or expires too soon")
		}
	case TLSReference:
		if len(material.Headers) != 0 || material.TLS == nil {
			return errors.New("TLS material must contain tls only")
		}
		_, err := validateTLS(tlsDocument{
			CAFile: material.TLS.CAFile, ClientCertificateFile: material.TLS.ClientCertificateFile,
			ClientKeyFile: material.TLS.ClientKeyFile, ServerName: material.TLS.ServerName,
		})
		if err != nil {
			return err
		}
		if !material.ExpiresAt.IsZero() && !material.ExpiresAt.After(time.Now().Add(minimumCredentialValidity)) {
			return errors.New("connection material is expired or expires too soon")
		}
	default:
		return errors.New("unsupported connection material reference kind")
	}
	return nil
}

func Validate(endpoint orchestrator.TargetEndpoint) error {
	if endpoint.Connection != nil && !endpoint.Connection.ExpiresAt.IsZero() && !endpoint.Connection.ExpiresAt.After(time.Now()) {
		return errors.New("target connection material has expired")
	}
	return nil
}

func cloneTarget(target orchestrator.EngineTarget) orchestrator.EngineTarget {
	result := target
	result.Endpoints = append([]orchestrator.TargetEndpoint(nil), target.Endpoints...)
	result.Handles = make(orchestrator.EngineHandles, len(target.Handles))
	for name, value := range target.Handles {
		result.Handles[name] = value
	}
	result.Metadata = cloneValues(target.Metadata)
	for index := range result.Endpoints {
		result.Endpoints[index].Metadata = cloneValues(result.Endpoints[index].Metadata)
		result.Endpoints[index].Connection = nil
	}
	return result
}

func cloneValues(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}

func earliest(left, right time.Time) time.Time {
	if left.IsZero() || !right.IsZero() && right.Before(left) {
		return right
	}
	return left
}

func referenceError(kind ReferenceKind, reference string, err error) error {
	switch {
	case errors.Is(err, ErrReferenceNotFound):
		return fmt.Errorf("resolve %s reference %q: reference was not found", kind, reference)
	case strings.Contains(err.Error(), "expired"):
		return fmt.Errorf("resolve %s reference %q: connection material is expired or expires too soon", kind, reference)
	default:
		return fmt.Errorf("resolve %s reference %q: connection material is unavailable", kind, reference)
	}
}
