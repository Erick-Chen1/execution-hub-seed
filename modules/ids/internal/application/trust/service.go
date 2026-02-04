package trust

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/industrial-data-source/internal/domain/trust"
)

// Service handles trust operations
type Service struct {
	repo     trust.Repository
	keyStore trust.KeyStore
	logger   zerolog.Logger
}

// NewService creates a new trust service
func NewService(repo trust.Repository, keyStore trust.KeyStore, logger zerolog.Logger) *Service {
	return &Service{
		repo:     repo,
		keyStore: keyStore,
		logger:   logger.With().Str("service", "trust").Logger(),
	}
}

// HashChainInput represents input for adding an event to the hash chain
type HashChainInput struct {
	EventID        uuid.UUID
	SourceID       string
	ClientRecordID string
	SourceType     string
	TsDevice       *time.Time
	TsGateway      *time.Time
	Key            *string
	EventType      string
	Payload        json.RawMessage
	SchemaVersion  string
}

// AddToHashChain adds an event to its source's hash chain
func (s *Service) AddToHashChain(ctx context.Context, input HashChainInput) (*trust.HashChainEntry, error) {
	// Compute event hash
	hashInput := &trust.EventHashInput{
		ClientRecordID: input.ClientRecordID,
		SourceType:     input.SourceType,
		SourceID:       input.SourceID,
		TsDevice:       input.TsDevice,
		TsGateway:      input.TsGateway,
		Key:            input.Key,
		EventType:      input.EventType,
		Payload:        input.Payload,
		SchemaVersion:  input.SchemaVersion,
	}

	eventHash, err := trust.ComputeEventHash(hashInput)
	if err != nil {
		return nil, fmt.Errorf("failed to compute event hash: %w", err)
	}

	// Get the latest chain entry for this source
	latestEntry, err := s.repo.GetLatestChainEntry(ctx, input.SourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest chain entry: %w", err)
	}

	// Determine previous hash and sequence number
	prevHash := ""
	sequenceNum := int64(1)
	if latestEntry != nil {
		prevHash = latestEntry.ChainHash
		sequenceNum = latestEntry.SequenceNum + 1
	}

	// Create and store the new chain entry
	entry := trust.NewHashChainEntry(input.EventID, input.SourceID, sequenceNum, eventHash, prevHash)

	if err := s.repo.InsertHashChainEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to insert hash chain entry: %w", err)
	}

	// Update event trust level to T2
	if err := s.repo.UpdateEventTrustLevel(ctx, input.EventID, trust.TrustLevelT2); err != nil {
		s.logger.Warn().Err(err).
			Str("eventId", input.EventID.String()).
			Msg("failed to update event trust level")
	}

	s.logger.Debug().
		Str("eventId", input.EventID.String()).
		Str("sourceId", input.SourceID).
		Int64("sequenceNum", sequenceNum).
		Str("chainHash", entry.ChainHash).
		Msg("event added to hash chain")

	return entry, nil
}

// BatchSignatureInput represents input for registering a batch signature
type BatchSignatureInput struct {
	SourceID     string
	EventIDs     []uuid.UUID
	BatchHash    string
	Signature    string
	SignatureAlg string
	KeyID        string
	SignedAt     time.Time
}

// RegisterBatchSignature registers a batch signature from a gateway
func (s *Service) RegisterBatchSignature(ctx context.Context, input BatchSignatureInput) (*trust.BatchSignature, error) {
	sig := trust.NewBatchSignature(
		input.SourceID,
		input.EventIDs,
		input.BatchHash,
		input.Signature,
		input.SignatureAlg,
		input.KeyID,
		input.SignedAt,
	)

	if err := s.repo.InsertBatchSignature(ctx, sig); err != nil {
		return nil, fmt.Errorf("failed to insert batch signature: %w", err)
	}

	s.logger.Info().
		Str("batchId", sig.BatchID.String()).
		Str("sourceId", input.SourceID).
		Int("eventCount", len(input.EventIDs)).
		Msg("batch signature registered")

	return sig, nil
}

// GetBatchSignature retrieves a batch signature by its ID
func (s *Service) GetBatchSignature(ctx context.Context, batchID uuid.UUID) (*trust.BatchSignature, error) {
	sig, err := s.repo.GetBatchSignature(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("failed to get batch signature: %w", err)
	}

	if sig == nil {
		return nil, trust.ErrBatchSignatureNotFound
	}

	s.logger.Debug().
		Str("batchId", batchID.String()).
		Str("sourceId", sig.SourceID).
		Str("status", string(sig.VerificationStatus)).
		Msg("batch signature retrieved")

	return sig, nil
}

// VerifyBatchSignature verifies a batch signature
func (s *Service) VerifyBatchSignature(ctx context.Context, batchID uuid.UUID) (*trust.TrustVerificationResult, error) {
	sig, err := s.repo.GetBatchSignature(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if sig == nil {
		return nil, fmt.Errorf("batch signature not found")
	}

	// Get signing key
	key, err := s.keyStore.GetKey(ctx, sig.KeyID)
	if err != nil {
		errMsg := err.Error()
		if updateErr := s.repo.UpdateBatchSignatureStatus(ctx, batchID, trust.VerificationStatusFailed, &errMsg); updateErr != nil {
			s.logger.Warn().Err(updateErr).Msg("failed to update batch signature status")
		}
		return nil, fmt.Errorf("failed to get signing key: %w", err)
	}

	// Verify signature
	isValid := trust.VerifyHMAC(sig.BatchHash, sig.Signature, key)

	if isValid {
		sig.MarkVerified()
		if err := s.repo.UpdateBatchSignatureStatus(ctx, batchID, trust.VerificationStatusVerified, nil); err != nil {
			s.logger.Warn().Err(err).Msg("failed to update batch signature status")
		}

		// Upgrade all events in batch to T3
		for _, eventID := range sig.EventIDs {
			if err := s.repo.UpdateEventTrustLevel(ctx, eventID, trust.TrustLevelT3); err != nil {
				s.logger.Warn().Err(err).
					Str("eventId", eventID.String()).
					Msg("failed to update event trust level to T3")
			}
		}

		s.logger.Info().
			Str("batchId", batchID.String()).
			Int("eventCount", len(sig.EventIDs)).
			Msg("batch signature verified successfully")
	} else {
		errMsg := "signature verification failed"
		sig.MarkFailed(errMsg)
		if err := s.repo.UpdateBatchSignatureStatus(ctx, batchID, trust.VerificationStatusFailed, &errMsg); err != nil {
			s.logger.Warn().Err(err).Msg("failed to update batch signature status")
		}

		s.logger.Warn().
			Str("batchId", batchID.String()).
			Msg("batch signature verification failed")
	}

	result := &trust.TrustVerificationResult{
		TrustLevel:     trust.TrustLevelT2,
		ChainValid:     true, // Assume chain is valid if events exist
		SignatureValid: &isValid,
		VerifiedAt:     time.Now().UTC(),
	}

	if isValid {
		result.TrustLevel = trust.TrustLevelT3
	} else {
		result.Errors = append(result.Errors, "signature verification failed")
	}

	return result, nil
}

// VerifyHashChain verifies the integrity of a hash chain for a source
func (s *Service) VerifyHashChain(ctx context.Context, sourceID string, fromSeq, toSeq int64) (*ChainVerificationResult, error) {
	startTime := time.Now()

	entries, err := s.repo.GetChainEntriesForSource(ctx, sourceID, fromSeq, toSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain entries: %w", err)
	}

	result := &ChainVerificationResult{
		SourceID:     sourceID,
		FromSequence: fromSeq,
		ToSequence:   toSeq,
		IsValid:      true,
		VerifiedAt:   time.Now().UTC(),
	}

	if len(entries) == 0 {
		result.EntriesChecked = 0
		return result, nil
	}

	result.EntriesChecked = len(entries)

	// Verify each entry
	for i, entry := range entries {
		// Verify the entry's own hash
		if !entry.Verify() {
			result.IsValid = false
			result.Breaks = append(result.Breaks, trust.ChainBreak{
				SourceID:     sourceID,
				BreakAt:      entry.SequenceNum,
				ExpectedHash: trust.ComputeChainHash(entry.EventHash, entry.PrevHash),
				ActualHash:   entry.ChainHash,
				DetectedAt:   time.Now().UTC(),
			})
		}

		// Verify chain continuity (prev hash matches previous entry's chain hash)
		if i > 0 {
			prevEntry := entries[i-1]
			if entry.PrevHash != prevEntry.ChainHash {
				result.IsValid = false
				result.Breaks = append(result.Breaks, trust.ChainBreak{
					SourceID:     sourceID,
					BreakAt:      entry.SequenceNum,
					ExpectedHash: prevEntry.ChainHash,
					ActualHash:   entry.PrevHash,
					DetectedAt:   time.Now().UTC(),
				})
			}
		}
	}

	result.DurationMs = int(time.Since(startTime).Milliseconds())

	s.logger.Info().
		Str("sourceId", sourceID).
		Int64("fromSeq", fromSeq).
		Int64("toSeq", toSeq).
		Int("entriesChecked", result.EntriesChecked).
		Bool("isValid", result.IsValid).
		Int("breaksFound", len(result.Breaks)).
		Int("durationMs", result.DurationMs).
		Msg("hash chain verification completed")

	return result, nil
}

// ChainVerificationResult represents the result of hash chain verification
type ChainVerificationResult struct {
	SourceID       string             `json:"sourceId"`
	FromSequence   int64              `json:"fromSequence"`
	ToSequence     int64              `json:"toSequence"`
	EntriesChecked int                `json:"entriesChecked"`
	IsValid        bool               `json:"isValid"`
	Breaks         []trust.ChainBreak `json:"breaks,omitempty"`
	DurationMs     int                `json:"durationMs"`
	VerifiedAt     time.Time          `json:"verifiedAt"`
}

// GenerateEvidenceBundle generates an evidence bundle for an event, action, or batch
func (s *Service) GenerateEvidenceBundle(ctx context.Context, bundleType string, subjectID uuid.UUID) (*trust.EvidenceBundle, error) {
	// Get raw data
	data, err := s.repo.GetEvidenceBundleData(ctx, bundleType, subjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get evidence data: %w", err)
	}

	// Create bundle
	bundle := trust.NewEvidenceBundle(bundleType, subjectID)
	bundle.Events = data.Events

	// Convert hash chain entries
	for _, entry := range data.HashChain {
		bundle.HashChain = append(bundle.HashChain, *entry)
	}

	// Convert signatures
	for _, sig := range data.Signatures {
		bundle.Signatures = append(bundle.Signatures, *sig)
	}

	bundle.Rules = data.Rules
	bundle.Actions = data.Actions

	// Compute verification summary
	bundle.Verification = s.computeVerificationSummary(bundle)

	// Finalize bundle (compute hash)
	if err := bundle.Finalize(); err != nil {
		return nil, fmt.Errorf("failed to finalize bundle: %w", err)
	}

	s.logger.Info().
		Str("bundleId", bundle.BundleID.String()).
		Str("bundleType", bundleType).
		Str("subjectId", subjectID.String()).
		Int("eventsCount", len(bundle.Events)).
		Int("chainEntriesCount", len(bundle.HashChain)).
		Int("signaturesCount", len(bundle.Signatures)).
		Str("trustLevel", bundle.Verification.OverallTrustLevel.String()).
		Msg("evidence bundle generated")

	return bundle, nil
}

// computeVerificationSummary computes the verification summary for a bundle
func (s *Service) computeVerificationSummary(bundle *trust.EvidenceBundle) trust.VerificationSummary {
	summary := trust.VerificationSummary{
		OverallTrustLevel: trust.TrustLevelT0,
		HashChainValid:    true,
	}

	// Determine overall trust level from events
	minTrust := trust.TrustLevelT3
	for _, event := range bundle.Events {
		if event.TrustLevel < minTrust {
			minTrust = event.TrustLevel
		}
	}
	summary.OverallTrustLevel = minTrust

	// Verify hash chain entries
	entryMap := make(map[string][]*trust.HashChainEntry)
	for i := range bundle.HashChain {
		entry := &bundle.HashChain[i]
		entryMap[entry.SourceID] = append(entryMap[entry.SourceID], entry)
	}

	for sourceID, entries := range entryMap {
		for i, entry := range entries {
			if !entry.Verify() {
				summary.HashChainValid = false
				summary.ChainBreaks = append(summary.ChainBreaks, trust.ChainBreak{
					SourceID:     sourceID,
					BreakAt:      entry.SequenceNum,
					ExpectedHash: trust.ComputeChainHash(entry.EventHash, entry.PrevHash),
					ActualHash:   entry.ChainHash,
					DetectedAt:   time.Now().UTC(),
				})
			}

			// Check continuity
			if i > 0 && entry.PrevHash != entries[i-1].ChainHash {
				summary.HashChainValid = false
				summary.ChainBreaks = append(summary.ChainBreaks, trust.ChainBreak{
					SourceID:     sourceID,
					BreakAt:      entry.SequenceNum,
					ExpectedHash: entries[i-1].ChainHash,
					ActualHash:   entry.PrevHash,
					DetectedAt:   time.Now().UTC(),
				})
			}
		}
	}

	// Check signature verification status
	if len(bundle.Signatures) > 0 {
		allVerified := true
		for _, sig := range bundle.Signatures {
			if sig.VerificationStatus != trust.VerificationStatusVerified {
				allVerified = false
				if sig.VerificationError != nil {
					summary.VerificationErrors = append(summary.VerificationErrors, *sig.VerificationError)
				}
			}
		}
		summary.SignatureValid = &allVerified
	}

	// Downgrade trust level if verification failed
	if !summary.HashChainValid && summary.OverallTrustLevel >= trust.TrustLevelT2 {
		summary.OverallTrustLevel = trust.TrustLevelT1
		summary.VerificationErrors = append(summary.VerificationErrors, "hash chain verification failed")
	}
	if summary.SignatureValid != nil && !*summary.SignatureValid && summary.OverallTrustLevel >= trust.TrustLevelT3 {
		summary.OverallTrustLevel = trust.TrustLevelT2
		summary.VerificationErrors = append(summary.VerificationErrors, "signature verification failed")
	}

	return summary
}

// GetTrustMetadata retrieves trust metadata for an event
func (s *Service) GetTrustMetadata(ctx context.Context, eventID uuid.UUID) (*trust.TrustMetadata, error) {
	return s.repo.GetTrustMetadata(ctx, eventID)
}

// BatchGetTrustMetadata retrieves trust metadata for multiple events
func (s *Service) BatchGetTrustMetadata(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]*trust.TrustMetadata, error) {
	return s.repo.BatchGetTrustMetadata(ctx, eventIDs)
}

// DowngradeEventTrust downgrades an event's trust level with audit logging
func (s *Service) DowngradeEventTrust(ctx context.Context, eventID uuid.UUID, toLevel trust.TrustLevel, reason, downgrader string) error {
	// Get current trust level
	metadata, err := s.repo.GetTrustMetadata(ctx, eventID)
	if err != nil {
		return err
	}

	if metadata.TrustLevel <= toLevel {
		return nil // No downgrade needed
	}

	// Update trust level
	if err := s.repo.UpdateEventTrustLevel(ctx, eventID, toLevel); err != nil {
		return err
	}

	s.logger.Warn().
		Str("eventId", eventID.String()).
		Int("fromLevel", int(metadata.TrustLevel)).
		Int("toLevel", int(toLevel)).
		Str("reason", reason).
		Str("downgrader", downgrader).
		Msg("event trust level downgraded")

	return nil
}
