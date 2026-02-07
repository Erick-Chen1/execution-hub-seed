package orchestrator

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/Knetic/govaluate"
)

// EvaluateCondition evaluates a condition expression against a JSON context.
// Empty condition returns true. Supports "true"/"false" literals.
func EvaluateCondition(condition string, contextJSON json.RawMessage) (bool, error) {
	cond := strings.TrimSpace(condition)
	if cond == "" {
		return true, nil
	}
	switch strings.ToLower(cond) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}

	params := buildContextParams(contextJSON)
	expr, err := govaluate.NewEvaluableExpression(cond)
	if err != nil {
		return false, err
	}
	result, err := expr.Evaluate(params)
	if err != nil {
		return false, err
	}
	switch v := result.(type) {
	case bool:
		return v, nil
	default:
		return false, errors.New("condition did not evaluate to boolean")
	}
}

func buildContextParams(contextJSON json.RawMessage) map[string]interface{} {
	params := map[string]interface{}{}
	if len(contextJSON) == 0 {
		return params
	}
	var raw interface{}
	if err := json.Unmarshal(contextJSON, &raw); err != nil {
		return params
	}
	if m, ok := raw.(map[string]interface{}); ok {
		for k, v := range m {
			params[k] = v
		}
		flattenContext("", m, params)
	}
	return params
}

func flattenContext(prefix string, m map[string]interface{}, out map[string]interface{}) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch vv := v.(type) {
		case map[string]interface{}:
			flattenContext(key, vv, out)
		default:
			out[key] = vv
		}
	}
}
