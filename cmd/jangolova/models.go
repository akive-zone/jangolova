package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"jangolova/internal/grimlock"
	"jangolova/targetconn"
)

func modelsCommand(args []string) error {
	flags := flag.NewFlagSet("models", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	jsonOutput := flags.Bool("json", false, "write the model protocol inventory as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("models accepts flags only")
	}

	runtime, err := grimlock.NewDefaultRuntime(targetconn.DefaultResolver())
	if err != nil {
		return err
	}
	protocols := runtime.ModelProtocols()

	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"apiVersion": grimlock.ModelAPIVersion,
			"protocols":  protocols,
		})
	}
	for _, protocol := range protocols {
		fmt.Fprintf(os.Stdout, "%s\tregistered\n", protocol)
	}
	return nil
}

func connectModelCommand(args []string) error {
	flags := flag.NewFlagSet("connect-model", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	protocol := flags.String("protocol", grimlock.OpenAICompatibleProtocol, "Grimlock model connector protocol")
	endpoint := flags.String("endpoint", "", "model provider HTTP(S) endpoint URL")
	modelName := flags.String("model", "", "model name (e.g. gpt-4o, gemini-1.5-flash)")
	profileID := flags.String("profile-id", "cli-model", "model profile ID")
	credentialRef := flags.String("credential-ref", "cli-credential", "opaque credential reference name")
	tlsRef := flags.String("tls-ref", "", "optional opaque TLS reference name")
	token := flags.String("token", "", "bearer token for inline credential; falls back to JANGOLOVA_CREDENTIAL_<REF> env")
	timeout := flags.Duration("timeout", 15*time.Second, "maximum connection timeout")
	jsonOutput := flags.Bool("json", false, "write connection result as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("connect-model accepts flags only")
	}
	if strings.TrimSpace(*endpoint) == "" {
		return errors.New("--endpoint is required")
	}
	if strings.TrimSpace(*modelName) == "" {
		return errors.New("--model is required")
	}
	if *timeout <= 0 {
		return errors.New("--timeout must be positive")
	}

	profile := grimlock.ModelProfile{
		APIVersion:    grimlock.ModelAPIVersion,
		ProfileID:     strings.TrimSpace(*profileID),
		Protocol:      strings.TrimSpace(*protocol),
		Endpoint:      strings.TrimSpace(*endpoint),
		Model:         strings.TrimSpace(*modelName),
		CredentialRef: strings.TrimSpace(*credentialRef),
		TLSRef:        strings.TrimSpace(*tlsRef),
	}
	if err := profile.Validate(); err != nil {
		return err
	}

	resolver := inlineOrDefaultResolver(profile.CredentialRef, strings.TrimSpace(*token))

	runtime, err := grimlock.NewDefaultRuntime(resolver)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	spec := grimlock.AgentSpec{
		SessionID: "cli-probe",
		Model:     profile,
	}

	session, err := runtime.CreateAgent(ctx, spec, nil)
	if err != nil {
		return fmt.Errorf("connect model: %w", err)
	}
	defer session.Close(context.Background())

	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"apiVersion":    grimlock.ModelAPIVersion,
			"profileId":     profile.ProfileID,
			"protocol":      profile.Protocol,
			"endpoint":      profile.Endpoint,
			"model":         profile.Model,
			"credentialRef": profile.CredentialRef,
			"status":        "connected",
		})
	}

	fmt.Fprintf(os.Stdout, "%s\t%s\t%s\tconnected\n", profile.Protocol, profile.Model, profile.Endpoint)
	return nil
}

// inlineOrDefaultResolver builds a resolver chain. When a raw bearer token is
// supplied via --token, it is injected as in-memory credential material for the
// named credentialRef so callers can probe a model provider without writing a
// JSON credential document first. All other references still fall through to
// the environment/directory DefaultResolver.
func inlineOrDefaultResolver(credRef, bearerToken string) targetconn.Resolver {
	if bearerToken == "" {
		return targetconn.DefaultResolver()
	}
	header := bearerToken
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		header = "Bearer " + header
	}
	// Credential material requires ExpiresAt to be set and the value to be
	// non-zero (validateResolvedMaterial in targetconn/prepare.go).
	// Use a one-hour inline lease — this is a CLI probe, not a long-lived session.
	expiry := time.Now().Add(1 * time.Hour)
	inline := targetconn.ResolverFunc(func(_ context.Context, req targetconn.Request) (targetconn.Material, error) {
		if req.Kind == targetconn.CredentialReference && req.Reference == credRef {
			return targetconn.Material{
				Headers:   map[string]string{"Authorization": header},
				ExpiresAt: expiry,
			}, nil
		}
		return targetconn.Material{}, targetconn.ErrReferenceNotFound
	})
	return targetconn.Chain{inline, targetconn.DefaultResolver()}
}
