package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/execution-hub/execution-hub/internal/domain/trust"
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
