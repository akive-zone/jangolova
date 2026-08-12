package grimlock

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/toolutils"
	"google.golang.org/genai"

	"jangolova/internal/blockade"
)

// BlockadeTools exposes read-only pixel observation to a Grimlock agent. The
// worker remains caller/deployment-owned; Grimlock only receives a client.
func BlockadeTools(client blockade.Client) ([]tool.Tool, error) {
	if client.BaseURL == "" {
		return nil, errors.New("Grimlock Blockade endpoint is required")
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"type":"object","additionalProperties":false,"required":["imageBase64"],"properties":{"imageBase64":{"type":"string","description":"Base64-encoded PNG or JPEG image."},"prompt":{"type":"string"}}}`), &schema); err != nil {
		return nil, err
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, err
	}
	return []tool.Tool{&blockadeTool{client: client, schema: resolved}}, nil
}

type blockadeTool struct {
	client blockade.Client
	schema *jsonschema.Resolved
}

var _ tool.Tool = (*blockadeTool)(nil)
var _ interface {
	Declaration() *genai.FunctionDeclaration
	ProcessRequest(agent.Context, *model.LLMRequest) error
	Run(agent.Context, any) (map[string]any, error)
} = (*blockadeTool)(nil)

func (t *blockadeTool) Name() string { return "blockade_observe" }
func (t *blockadeTool) Description() string {
	return "Observe an image through Blockade and return detected objects and masks."
}
func (t *blockadeTool) IsLongRunning() bool { return false }
func (t *blockadeTool) Declaration() *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{Name: t.Name(), Description: t.Description(), ParametersJsonSchema: t.schema.Schema()}
}
func (t *blockadeTool) ProcessRequest(_ agent.Context, request *model.LLMRequest) error {
	return toolutils.PackTool(request, t)
}

func (t *blockadeTool) Run(ctx agent.Context, arguments any) (map[string]any, error) {
	input, ok := arguments.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Blockade tool expected object arguments, got %T", arguments)
	}
	if err := t.schema.Validate(input); err != nil {
		return nil, err
	}
	encoded, _ := input["imageBase64"].(string)
	image, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode Blockade image: %w", err)
	}
	prompt, _ := input["prompt"].(string)
	result, err := t.client.Observe(ctx, blockade.ObserveRequest{Image: image, Prompt: prompt})
	if err != nil {
		return nil, err
	}
	return map[string]any{"apiVersion": result.APIVersion, "requestId": result.RequestID, "observations": result.Observations}, nil
}
