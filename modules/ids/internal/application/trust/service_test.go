package trust

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/industrial-data-source/internal/domain/trust"
)

// MockRepository is a mock implementation of trust.Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) InsertHashChainEntry(ctx context.Context, entry *trust.HashChainEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockRepository) GetHashChainEntry(ctx context.Context, eventID uuid.UUID) (*trust.HashChainEntry, error) {
	args := m.Called(ctx, eventID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*trust.HashChainEntry), args.Error(1)
}

func (m *MockRepository) GetLatestChainEntry(ctx context.Context, sourceID string) (*trust.HashChainEntry, error) {
	args := m.Called(ctx, sourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*trust.HashChainEntry), args.Error(1)
}

func (m *MockRepository) GetChainEntriesForSource(ctx context.Context, sourceID string, fromSeq, toSeq int64) ([]*trust.HashChainEntry, error) {
	args := m.Called(ctx, sourceID, fromSeq, toSeq)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*trust.HashChainEntry), args.Error(1)
}

func (m *MockRepository) GetChainEntriesForEvents(ctx context.Context, eventIDs []uuid.UUID) ([]*trust.HashChainEntry, error) {
	args := m.Called(ctx, eventIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*trust.HashChainEntry), args.Error(1)
}

func (m *MockRepository) InsertBatchSignature(ctx context.Context, sig *trust.BatchSignature) error {
	args := m.Called(ctx, sig)
	return args.Error(0)
}

func (m *MockRepository) GetBatchSignature(ctx context.Context, batchID uuid.UUID) (*trust.BatchSignature, error) {
	args := m.Called(ctx, batchID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*trust.BatchSignature), args.Error(1)
}

func (m *MockRepository) GetBatchSignaturesBySource(ctx context.Context, sourceID string, limit int) ([]*trust.BatchSignature, error) {
	args := m.Called(ctx, sourceID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*trust.BatchSignature), args.Error(1)
}

func (m *MockRepository) GetBatchSignatureForEvent(ctx context.Context, eventID uuid.UUID) (*trust.BatchSignature, error) {
	args := m.Called(ctx, eventID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*trust.BatchSignature), args.Error(1)
}

func (m *MockRepository) UpdateBatchSignatureStatus(ctx context.Context, batchID uuid.UUID, status trust.VerificationStatus, errMsg *string) error {
	args := m.Called(ctx, batchID, status, errMsg)
	return args.Error(0)
}

func (m *MockRepository) GetTrustMetadata(ctx context.Context, eventID uuid.UUID) (*trust.TrustMetadata, error) {
	args := m.Called(ctx, eventID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*trust.TrustMetadata), args.Error(1)
}

func (m *MockRepository) BatchGetTrustMetadata(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]*trust.TrustMetadata, error) {
	args := m.Called(ctx, eventIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID]*trust.TrustMetadata), args.Error(1)
}

func (m *MockRepository) UpdateEventTrustLevel(ctx context.Context, eventID uuid.UUID, level trust.TrustLevel) error {
	args := m.Called(ctx, eventID, level)
	return args.Error(0)
}

func (m *MockRepository) GetEvidenceBundleData(ctx context.Context, bundleType string, subjectID uuid.UUID) (*trust.EvidenceBundleData, error) {
	args := m.Called(ctx, bundleType, subjectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*trust.EvidenceBundleData), args.Error(1)
}

// MockKeyStore is a mock implementation of trust.KeyStore
type MockKeyStore struct {
	mock.Mock
}

func (m *MockKeyStore) GetKey(ctx context.Context, keyID string) ([]byte, error) {
	args := m.Called(ctx, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockKeyStore) GetKeyForSource(ctx context.Context, sourceID string) (string, []byte, error) {
	args := m.Called(ctx, sourceID)
	if args.Get(1) == nil {
		return args.String(0), nil, args.Error(2)
	}
	return args.String(0), args.Get(1).([]byte), args.Error(2)
}

func TestService_AddToHashChain(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	t.Run("adds first entry to chain (genesis)", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockKeyStore := new(MockKeyStore)
		svc := NewService(mockRepo, mockKeyStore, logger)

		eventID := uuid.New()
		sourceID := "gateway-001"

		input := HashChainInput{
			EventID:        eventID,
			SourceID:       sourceID,
			ClientRecordID: "record-001",
			SourceType:     "GW",
			EventType:      "SENSOR_READING",
			Payload:        json.RawMessage(`{"temperature": 25.5}`),
			SchemaVersion:  "1.0.0",
		}

		// No previous entry (genesis)
		mockRepo.On("GetLatestChainEntry", ctx, sourceID).Return(nil, nil)
		mockRepo.On("InsertHashChainEntry", ctx, mock.AnythingOfType("*trust.HashChainEntry")).Return(nil)
		mockRepo.On("UpdateEventTrustLevel", ctx, eventID, trust.TrustLevelT2).Return(nil)

		entry, err := svc.AddToHashChain(ctx, input)

		require.NoError(t, err)
		assert.Equal(t, eventID, entry.EventID)
		assert.Equal(t, sourceID, entry.SourceID)
		assert.Equal(t, int64(1), entry.SequenceNum)
		assert.Empty(t, entry.PrevHash)
		assert.NotEmpty(t, entry.EventHash)
		assert.NotEmpty(t, entry.ChainHash)
		assert.Equal(t, trust.TrustLevelT2, entry.TrustLevel)

		mockRepo.AssertExpectations(t)
	})

	t.Run("adds entry to existing chain", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockKeyStore := new(MockKeyStore)
		svc := NewService(mockRepo, mockKeyStore, logger)

		eventID := uuid.New()
		sourceID := "gateway-001"

		prevEntry := &trust.HashChainEntry{
			ID:          1,
			EventID:     uuid.New(),
			SourceID:    sourceID,
			SequenceNum: 5,
			EventHash:   "prevEventHash",
			PrevHash:    "prevPrevHash",
			ChainHash:   "prevChainHash123",
			TrustLevel:  trust.TrustLevelT2,
			CreatedAt:   time.Now().UTC(),
		}

		input := HashChainInput{
			EventID:        eventID,
			SourceID:       sourceID,
			ClientRecordID: "record-002",
			SourceType:     "GW",
			EventType:      "SENSOR_READING",
			Payload:        json.RawMessage(`{"temperature": 30.0}`),
			SchemaVersion:  "1.0.0",
		}

		mockRepo.On("GetLatestChainEntry", ctx, sourceID).Return(prevEntry, nil)
		mockRepo.On("InsertHashChainEntry", ctx, mock.AnythingOfType("*trust.HashChainEntry")).Return(nil)
		mockRepo.On("UpdateEventTrustLevel", ctx, eventID, trust.TrustLevelT2).Return(nil)

		entry, err := svc.AddToHashChain(ctx, input)

		require.NoError(t, err)
		assert.Equal(t, eventID, entry.EventID)
		assert.Equal(t, int64(6), entry.SequenceNum)
		assert.Equal(t, prevEntry.ChainHash, entry.PrevHash)

		mockRepo.AssertExpectations(t)
	})
}

func TestService_RegisterBatchSignature(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockRepo := new(MockRepository)
	mockKeyStore := new(MockKeyStore)
	svc := NewService(mockRepo, mockKeyStore, logger)

	eventIDs := []uuid.UUID{uuid.New(), uuid.New()}
	input := BatchSignatureInput{
		SourceID:     "gateway-001",
		EventIDs:     eventIDs,
		BatchHash:    "batchHash123",
		Signature:    "signature456",
		SignatureAlg: "HMAC-SHA256",
		KeyID:        "key-001",
		SignedAt:     time.Now().UTC(),
	}

	mockRepo.On("InsertBatchSignature", ctx, mock.AnythingOfType("*trust.BatchSignature")).Return(nil)

	sig, err := svc.RegisterBatchSignature(ctx, input)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, sig.BatchID)
	assert.Equal(t, input.SourceID, sig.SourceID)
	assert.Equal(t, input.EventIDs, sig.EventIDs)
	assert.Equal(t, trust.VerificationStatusPending, sig.VerificationStatus)

	mockRepo.AssertExpectations(t)
}

func TestService_VerifyBatchSignature(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	t.Run("verifies valid signature", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockKeyStore := new(MockKeyStore)
		svc := NewService(mockRepo, mockKeyStore, logger)

		batchID := uuid.New()
		eventIDs := []uuid.UUID{uuid.New(), uuid.New()}
		key := []byte("test-secret-key")
		batchHash := "testBatchHash"
		signature := trust.CreateHMAC(batchHash, key)

		sig := &trust.BatchSignature{
			BatchID:            batchID,
			SourceID:           "gateway-001",
			EventIDs:           eventIDs,
			BatchHash:          batchHash,
			Signature:          signature,
			SignatureAlg:       "HMAC-SHA256",
			KeyID:              "key-001",
			SignedAt:           time.Now().UTC(),
			VerificationStatus: trust.VerificationStatusPending,
		}

		mockRepo.On("GetBatchSignature", ctx, batchID).Return(sig, nil)
		mockKeyStore.On("GetKey", ctx, "key-001").Return(key, nil)
		mockRepo.On("UpdateBatchSignatureStatus", ctx, batchID, trust.VerificationStatusVerified, (*string)(nil)).Return(nil)
		mockRepo.On("UpdateEventTrustLevel", ctx, mock.AnythingOfType("uuid.UUID"), trust.TrustLevelT3).Return(nil)

		result, err := svc.VerifyBatchSignature(ctx, batchID)

		require.NoError(t, err)
		assert.Equal(t, trust.TrustLevelT3, result.TrustLevel)
		assert.True(t, result.ChainValid)
		assert.NotNil(t, result.SignatureValid)
		assert.True(t, *result.SignatureValid)
		assert.Empty(t, result.Errors)

		mockRepo.AssertExpectations(t)
		mockKeyStore.AssertExpectations(t)
	})

	t.Run("fails with invalid signature", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockKeyStore := new(MockKeyStore)
		svc := NewService(mockRepo, mockKeyStore, logger)

		batchID := uuid.New()
		key := []byte("test-secret-key")

		sig := &trust.BatchSignature{
			BatchID:            batchID,
			SourceID:           "gateway-001",
			EventIDs:           []uuid.UUID{uuid.New()},
			BatchHash:          "batchHash",
			Signature:          "invalidSignature",
			SignatureAlg:       "HMAC-SHA256",
			KeyID:              "key-001",
			SignedAt:           time.Now().UTC(),
			VerificationStatus: trust.VerificationStatusPending,
		}

		mockRepo.On("GetBatchSignature", ctx, batchID).Return(sig, nil)
		mockKeyStore.On("GetKey", ctx, "key-001").Return(key, nil)
		mockRepo.On("UpdateBatchSignatureStatus", ctx, batchID, trust.VerificationStatusFailed, mock.AnythingOfType("*string")).Return(nil)

		result, err := svc.VerifyBatchSignature(ctx, batchID)

		require.NoError(t, err)
		assert.Equal(t, trust.TrustLevelT2, result.TrustLevel)
		assert.NotNil(t, result.SignatureValid)
		assert.False(t, *result.SignatureValid)
		assert.NotEmpty(t, result.Errors)

		mockRepo.AssertExpectations(t)
		mockKeyStore.AssertExpectations(t)
	})
}

func TestService_VerifyHashChain(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	t.Run("verifies valid chain", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockKeyStore := new(MockKeyStore)
		svc := NewService(mockRepo, mockKeyStore, logger)

		sourceID := "gateway-001"

		// Create valid chain entries
		entry1 := trust.NewHashChainEntry(uuid.New(), sourceID, 1, "eventHash1", "")
		entry2 := trust.NewHashChainEntry(uuid.New(), sourceID, 2, "eventHash2", entry1.ChainHash)
		entry3 := trust.NewHashChainEntry(uuid.New(), sourceID, 3, "eventHash3", entry2.ChainHash)

		entries := []*trust.HashChainEntry{entry1, entry2, entry3}

		mockRepo.On("GetChainEntriesForSource", ctx, sourceID, int64(1), int64(3)).Return(entries, nil)

		result, err := svc.VerifyHashChain(ctx, sourceID, 1, 3)

		require.NoError(t, err)
		assert.True(t, result.IsValid)
		assert.Equal(t, 3, result.EntriesChecked)
		assert.Empty(t, result.Breaks)

		mockRepo.AssertExpectations(t)
	})

	t.Run("detects broken chain", func(t *testing.T) {
		mockRepo := new(MockRepository)
		mockKeyStore := new(MockKeyStore)
		svc := NewService(mockRepo, mockKeyStore, logger)

		sourceID := "gateway-001"

		// Create chain with a break
		entry1 := trust.NewHashChainEntry(uuid.New(), sourceID, 1, "eventHash1", "")
		entry2 := trust.NewHashChainEntry(uuid.New(), sourceID, 2, "eventHash2", entry1.ChainHash)
		entry3 := trust.NewHashChainEntry(uuid.New(), sourceID, 3, "eventHash3", "wrongPrevHash") // Break!

		entries := []*trust.HashChainEntry{entry1, entry2, entry3}

		mockRepo.On("GetChainEntriesForSource", ctx, sourceID, int64(1), int64(3)).Return(entries, nil)

		result, err := svc.VerifyHashChain(ctx, sourceID, 1, 3)

		require.NoError(t, err)
		assert.False(t, result.IsValid)
		assert.Equal(t, 3, result.EntriesChecked)
		assert.NotEmpty(t, result.Breaks)
		assert.Equal(t, int64(3), result.Breaks[0].BreakAt)

		mockRepo.AssertExpectations(t)
	})
}

func TestService_GenerateEvidenceBundle(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockRepo := new(MockRepository)
	mockKeyStore := new(MockKeyStore)
	svc := NewService(mockRepo, mockKeyStore, logger)

	eventID := uuid.New()
	sourceID := "gateway-001"

	eventEvidence := trust.EventEvidence{
		EventID:        eventID,
		ClientRecordID: "record-001",
		SourceType:     "GW",
		SourceID:       sourceID,
		TsServer:       time.Now().UTC(),
		EventType:      "SENSOR_READING",
		Payload:        json.RawMessage(`{"temperature": 25.5}`),
		SchemaVersion:  "1.0.0",
		TrustLevel:     trust.TrustLevelT2,
	}

	chainEntry := trust.NewHashChainEntry(eventID, sourceID, 1, "eventHash", "")

	bundleData := &trust.EvidenceBundleData{
		Events:    []trust.EventEvidence{eventEvidence},
		HashChain: []*trust.HashChainEntry{chainEntry},
	}

	mockRepo.On("GetEvidenceBundleData", ctx, "EVENT", eventID).Return(bundleData, nil)

	bundle, err := svc.GenerateEvidenceBundle(ctx, "EVENT", eventID)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, bundle.BundleID)
	assert.Equal(t, "EVENT", bundle.BundleType)
	assert.Equal(t, eventID, bundle.SubjectID)
	assert.Len(t, bundle.Events, 1)
	assert.Len(t, bundle.HashChain, 1)
	assert.NotEmpty(t, bundle.BundleHash)
	assert.True(t, bundle.VerifyIntegrity())
	assert.Equal(t, trust.TrustLevelT2, bundle.Verification.OverallTrustLevel)
	assert.True(t, bundle.Verification.HashChainValid)

	mockRepo.AssertExpectations(t)
}

func TestService_GetTrustMetadata(t *testing.T) {
	ctx := context.Background()
	logger := zerolog.Nop()

	mockRepo := new(MockRepository)
	mockKeyStore := new(MockKeyStore)
	svc := NewService(mockRepo, mockKeyStore, logger)

	eventID := uuid.New()
	chainEntryID := int64(123)

	metadata := &trust.TrustMetadata{
		EventID:      eventID,
		TrustLevel:   trust.TrustLevelT2,
		ChainEntryID: &chainEntryID,
		Flags:        []string{"CHAIN_VERIFIED"},
	}

	mockRepo.On("GetTrustMetadata", ctx, eventID).Return(metadata, nil)

	result, err := svc.GetTrustMetadata(ctx, eventID)

	require.NoError(t, err)
	assert.Equal(t, eventID, result.EventID)
	assert.Equal(t, trust.TrustLevelT2, result.TrustLevel)
	assert.NotNil(t, result.ChainEntryID)
	assert.Equal(t, chainEntryID, *result.ChainEntryID)

	mockRepo.AssertExpectations(t)
}
