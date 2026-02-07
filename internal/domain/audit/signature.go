package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"
)

type signaturePayload struct {
	AuditID        string   `json:"auditId"`
	EntityType     string   `json:"entityType"`
	EntityID       string   `json:"entityId"`
	Action         string   `json:"action"`
	Actor          string   `json:"actor"`
	ActorRoles     []string `json:"actorRoles,omitempty"`
	ActorIP        string   `json:"actorIp,omitempty"`
	UserAgent      string   `json:"userAgent,omitempty"`
	OldValues      string   `json:"oldValues,omitempty"`
	NewValues      string   `json:"newValues,omitempty"`
	Diff           string   `json:"diff,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	RiskLevel      string   `json:"riskLevel"`
	Tags           []string `json:"tags,omitempty"`
	TraceID        string   `json:"traceId,omitempty"`
	SessionID      string   `json:"sessionId,omitempty"`
	RequestMethod  string   `json:"requestMethod,omitempty"`
	RequestPath    string   `json:"requestPath,omitempty"`
	ResponseStatus int      `json:"responseStatus,omitempty"`
	DurationMs     int      `json:"durationMs,omitempty"`
	CreatedAt      string   `json:"createdAt"`
}

func buildSignaturePayload(log *AuditLog) signaturePayload {
	payload := signaturePayload{
		AuditID:        log.AuditID.String(),
		EntityType:     string(log.EntityType),
		EntityID:       log.EntityID,
		Action:         string(log.Action),
		Actor:          log.Actor,
		ActorRoles:     log.ActorRoles,
		UserAgent:      log.UserAgent,
		Reason:         log.Reason,
		RiskLevel:      string(log.RiskLevel),
		Tags:           log.Tags,
		TraceID:        log.TraceID,
		SessionID:      log.SessionID,
		RequestMethod:  log.RequestMethod,
		RequestPath:    log.RequestPath,
		ResponseStatus: log.ResponseStatus,
		DurationMs:     log.DurationMs,
		CreatedAt:      log.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	if log.ActorIP != nil {
		payload.ActorIP = log.ActorIP.String()
	}
	if len(log.OldValues) > 0 {
		payload.OldValues = base64.StdEncoding.EncodeToString(log.OldValues)
	}
	if len(log.NewValues) > 0 {
		payload.NewValues = base64.StdEncoding.EncodeToString(log.NewValues)
	}
	if len(log.Diff) > 0 {
		payload.Diff = base64.StdEncoding.EncodeToString(log.Diff)
	}
	return payload
}

// SignAuditLog generates an HMAC signature for the audit log.
func SignAuditLog(log *AuditLog, key []byte) ([]byte, error) {
	payload := buildSignaturePayload(log)
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil), nil
}

// VerifyAuditLogSignature verifies the HMAC signature for the audit log.
func VerifyAuditLogSignature(log *AuditLog, key []byte) (bool, error) {
	if len(log.Signature) == 0 {
		return false, nil
	}
	expected, err := SignAuditLog(log, key)
	if err != nil {
		return false, err
	}
	return hmac.Equal(expected, log.Signature), nil
}
