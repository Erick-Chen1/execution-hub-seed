package trust

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustLevel_String(t *testing.T) {
	tests := []struct {
		level    TrustLevel
		expected string
	}{
		{TrustLevelT0, "T0"},
		{TrustLevelT1, "T1"},
		{TrustLevelT2, "T2"},
		{TrustLevelT3, "T3"},
		{TrustLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.level.String())
		})
	}
}

func TestParseTrustLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected TrustLevel
		hasError bool
	}{
		{"T0", TrustLevelT0, false},
		{"T1", TrustLevelT1, false},
		{"T2", TrustLevelT2, false},
		{"T3", TrustLevelT3, false},
		{"INVALID", TrustLevelT0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseTrustLevel(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, level)
			}
		})
	}
}

func TestComputeEventHash(t *testing.T) {
	payload := json.RawMessage(`{"temperature": 25.5}`)
	now := time.Now().UTC()
	key := "asset-001"

	input := &EventHashInput{
		ClientRecordID: "record-001",
		SourceType:     "GW",
		SourceID:       "gateway-001",
		TsDevice:       &now,
		Key:            &key,
		EventType:      "SENSOR_READING",
		Payload:        payload,
		SchemaVersion:  "1.0.0",
	}

	hash1, err := ComputeEventHash(input)
	require.NoError(t, err)
	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 64) // SHA-256 produces 64 hex characters

	// Same input should produce same hash
	hash2, err := ComputeEventHash(input)
	require.NoError(t, err)
	assert.Equal(t, hash1, hash2)

	// Different input should produce different hash
	input.Payload = json.RawMessage(`{"temperature": 30.0}`)
	hash3, err := ComputeEventHash(input)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash3)
}

func TestComputeChainHash(t *testing.T) {
	eventHash := "abc123"
	prevHash := "def456"

	chainHash := ComputeChainHash(eventHash, prevHash)
	assert.NotEmpty(t, chainHash)
	assert.Len(t, chainHash, 64)

	// Same inputs should produce same output
	chainHash2 := ComputeChainHash(eventHash, prevHash)
	assert.Equal(t, chainHash, chainHash2)

	// Different inputs should produce different output
	chainHash3 := ComputeChainHash(eventHash, "xyz789")
	assert.NotEqual(t, chainHash, chainHash3)
}

func TestNewHashChainEntry(t *testing.T) {
	eventID := uuid.New()
	sourceID := "gateway-001"
	sequenceNum := int64(1)
	eventHash := "abc123def456789012345678901234567890123456789012345678901234"
	prevHash := ""

	entry := NewHashChainEntry(eventID, sourceID, sequenceNum, eventHash, prevHash)

	assert.Equal(t, eventID, entry.EventID)
	assert.Equal(t, sourceID, entry.SourceID)
	assert.Equal(t, sequenceNum, entry.SequenceNum)
	assert.Equal(t, eventHash, entry.EventHash)
	assert.Equal(t, prevHash, entry.PrevHash)
	// Hash chain entries are always assigned TrustLevelT2 because:
	// - T2 = "Hash chain verified" - provides data integrity verification
	// - Hash chain can verify that data hasn't been tampered with (integrity)
	// - But it cannot verify data authenticity (who created it) - that requires T3 with signatures
	assert.Equal(t, TrustLevelT2, entry.TrustLevel, "hash chain provides T2 trust level (integrity verification only)")
	assert.NotEmpty(t, entry.ChainHash)
	assert.NotZero(t, entry.CreatedAt)
}

func TestHashChainEntry_Verify(t *testing.T) {
	eventID := uuid.New()
	sourceID := "gateway-001"
	eventHash := "abc123def456789012345678901234567890123456789012345678901234"
	prevHash := "prev12345678901234567890123456789012345678901234567890123456"

	t.Run("valid entry should verify", func(t *testing.T) {
		entry := NewHashChainEntry(eventID, sourceID, 1, eventHash, prevHash)
		assert.True(t, entry.Verify())
	})

	t.Run("tampered chain hash should fail", func(t *testing.T) {
		entry := NewHashChainEntry(eventID, sourceID, 1, eventHash, prevHash)
		entry.ChainHash = "tampered"
		assert.False(t, entry.Verify())
	})

	t.Run("tampered event hash should fail verification", func(t *testing.T) {
		entry := NewHashChainEntry(eventID, sourceID, 1, eventHash, prevHash)
		originalChainHash := entry.ChainHash
		// Tamper with EventHash - the stored ChainHash won't match recomputed hash
		entry.EventHash = "tampered56789012345678901234567890123456789012345678901234"
		// Verify that ChainHash no longer matches the recomputed value
		recomputedHash := ComputeChainHash(entry.EventHash, entry.PrevHash)
		assert.NotEqual(t, originalChainHash, recomputedHash)
		assert.False(t, entry.Verify())
	})

	t.Run("tampered prev hash should fail verification", func(t *testing.T) {
		entry := NewHashChainEntry(eventID, sourceID, 1, eventHash, prevHash)
		originalChainHash := entry.ChainHash
		// Tamper with PrevHash - the stored ChainHash won't match recomputed hash
		entry.PrevHash = "tampered5678901234567890123456789012345678901234567890123456"
		// Verify that ChainHash no longer matches the recomputed value
		recomputedHash := ComputeChainHash(entry.EventHash, entry.PrevHash)
		assert.NotEqual(t, originalChainHash, recomputedHash)
		assert.False(t, entry.Verify())
	})
}

func TestComputeBatchHash(t *testing.T) {
	// Use realistic SHA-256 format (64 hex characters) for test data
	eventHashes := []string{
		"abc123def456789012345678901234567890123456789012345678901234abcd",
		"def456789012345678901234567890123456789012345678901234abcdef1234",
		"789012345678901234567890123456789012345678901234abcdef123456789a",
	}

	batchHash := ComputeBatchHash(eventHashes)
	assert.NotEmpty(t, batchHash)
	assert.Len(t, batchHash, 64)

	// Same inputs should produce same output
	batchHash2 := ComputeBatchHash(eventHashes)
	assert.Equal(t, batchHash, batchHash2)

	// Different order should produce different output
	reorderedHashes := []string{eventHashes[1], eventHashes[0], eventHashes[2]}
	batchHash3 := ComputeBatchHash(reorderedHashes)
	assert.NotEqual(t, batchHash, batchHash3)
}

func TestHMAC(t *testing.T) {
	// Use realistic SHA-256 format (64 hex characters) for test data
	batchHash := "abc123def456789012345678901234567890123456789012345678901234abcd"
	key := []byte("secret-key-for-testing")

	// Create HMAC
	signature := CreateHMAC(batchHash, key)
	assert.NotEmpty(t, signature)

	// Verify should succeed with correct key
	assert.True(t, VerifyHMAC(batchHash, signature, key))

	// Verify should fail with wrong key
	assert.False(t, VerifyHMAC(batchHash, signature, []byte("wrong-key")))

	// Verify should fail with wrong data
	wrongHash := "def456789012345678901234567890123456789012345678901234abcdef1234"
	assert.False(t, VerifyHMAC(wrongHash, signature, key))

	// Verify should fail with invalid signature
	assert.False(t, VerifyHMAC(batchHash, "invalid-signature", key))

	// Edge case: empty batch hash should still produce valid HMAC
	emptyHashSignature := CreateHMAC("", key)
	assert.NotEmpty(t, emptyHashSignature)
	assert.True(t, VerifyHMAC("", emptyHashSignature, key))

	// Edge case: empty key should produce different signature
	emptyKeySignature := CreateHMAC(batchHash, []byte{})
	assert.NotEmpty(t, emptyKeySignature)
	assert.NotEqual(t, signature, emptyKeySignature)
}

func TestNewBatchSignature(t *testing.T) {
	sourceID := "gateway-001"
	eventIDs := []uuid.UUID{uuid.New(), uuid.New()}
	batchHash := "hash123"
	signature := "sig456"
	signatureAlg := "HMAC-SHA256"
	keyID := "key-001"
	signedAt := time.Now().UTC()

	sig := NewBatchSignature(sourceID, eventIDs, batchHash, signature, signatureAlg, keyID, signedAt)

	assert.NotEqual(t, uuid.Nil, sig.BatchID)
	assert.Equal(t, sourceID, sig.SourceID)
	assert.Equal(t, eventIDs, sig.EventIDs)
	assert.Equal(t, batchHash, sig.BatchHash)
	assert.Equal(t, signature, sig.Signature)
	assert.Equal(t, signatureAlg, sig.SignatureAlg)
	assert.Equal(t, keyID, sig.KeyID)
	assert.Equal(t, signedAt, sig.SignedAt)
	assert.Equal(t, VerificationStatusPending, sig.VerificationStatus)
	assert.Nil(t, sig.VerifiedAt)
}

func TestBatchSignature_MarkVerified(t *testing.T) {
	sig := NewBatchSignature("src", nil, "hash", "sig", "alg", "key", time.Now())
	
	sig.MarkVerified()
	
	assert.Equal(t, VerificationStatusVerified, sig.VerificationStatus)
	assert.NotNil(t, sig.VerifiedAt)
	assert.Nil(t, sig.VerificationError)
}

func TestBatchSignature_MarkFailed(t *testing.T) {
	sig := NewBatchSignature("src", nil, "hash", "sig", "alg", "key", time.Now())
	errMsg := "signature mismatch"
	
	sig.MarkFailed(errMsg)
	
	assert.Equal(t, VerificationStatusFailed, sig.VerificationStatus)
	// VerifiedAt should NOT be set on failure (semantic clarity: VerifiedAt means successful verification)
	assert.Nil(t, sig.VerifiedAt)
	assert.NotNil(t, sig.VerificationError)
	assert.Equal(t, errMsg, *sig.VerificationError)
}

func TestNewEvidenceBundle(t *testing.T) {
	bundleType := "EVENT"
	subjectID := uuid.New()

	bundle := NewEvidenceBundle(bundleType, subjectID)

	assert.NotEqual(t, uuid.Nil, bundle.BundleID)
	assert.Equal(t, bundleType, bundle.BundleType)
	assert.Equal(t, subjectID, bundle.SubjectID)
	assert.NotZero(t, bundle.GeneratedAt)
}

func TestEvidenceBundle_ComputeBundleHash(t *testing.T) {
	bundle := NewEvidenceBundle("EVENT", uuid.New())
	bundle.Events = []EventEvidence{
		{
			EventID:        uuid.New(),
			ClientRecordID: "record-001",
			SourceType:     "GW",
			SourceID:       "gateway-001",
			EventType:      "TEST",
			Payload:        json.RawMessage(`{}`),
			SchemaVersion:  "1.0.0",
			TrustLevel:     TrustLevelT2,
		},
	}

	hash, err := bundle.ComputeBundleHash()
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64)

	// Same bundle should produce same hash
	hash2, err := bundle.ComputeBundleHash()
	require.NoError(t, err)
	assert.Equal(t, hash, hash2)
}

func TestEvidenceBundle_FinalizeAndVerify(t *testing.T) {
	t.Run("valid bundle should verify after finalization", func(t *testing.T) {
		bundle := NewEvidenceBundle("EVENT", uuid.New())
		bundle.Events = []EventEvidence{
			{
				EventID:        uuid.New(),
				ClientRecordID: "record-001",
				SourceType:     "GW",
				SourceID:       "gateway-001",
				EventType:      "TEST",
				TsServer:       time.Now().UTC(),
				Payload:        json.RawMessage(`{}`),
				SchemaVersion:  "1.0.0",
				TrustLevel:     TrustLevelT2,
			},
		}

		// Finalize
		err := bundle.Finalize()
		require.NoError(t, err)
		assert.NotEmpty(t, bundle.BundleHash)

		// Verify should succeed
		assert.True(t, bundle.VerifyIntegrity())
	})

	t.Run("tampered event should fail verification", func(t *testing.T) {
		bundle := NewEvidenceBundle("EVENT", uuid.New())
		bundle.Events = []EventEvidence{
			{
				EventID:        uuid.New(),
				ClientRecordID: "record-001",
				SourceType:     "GW",
				SourceID:       "gateway-001",
				EventType:      "TEST",
				TsServer:       time.Now().UTC(),
				Payload:        json.RawMessage(`{}`),
				SchemaVersion:  "1.0.0",
				TrustLevel:     TrustLevelT2,
			},
		}

		err := bundle.Finalize()
		require.NoError(t, err)

		// Tampering should fail verification
		bundle.Events[0].ClientRecordID = "tampered"
		assert.False(t, bundle.VerifyIntegrity())
	})

	t.Run("tampered bundle hash should fail verification", func(t *testing.T) {
		bundle := NewEvidenceBundle("EVENT", uuid.New())
		bundle.Events = []EventEvidence{
			{
				EventID:        uuid.New(),
				ClientRecordID: "record-001",
				SourceType:     "GW",
				SourceID:       "gateway-001",
				EventType:      "TEST",
				TsServer:       time.Now().UTC(),
				Payload:        json.RawMessage(`{}`),
				SchemaVersion:  "1.0.0",
				TrustLevel:     TrustLevelT2,
			},
		}

		err := bundle.Finalize()
		require.NoError(t, err)

		// Tamper with BundleHash directly
		bundle.BundleHash = "tampered-hash-value"
		assert.False(t, bundle.VerifyIntegrity())
	})

	t.Run("tampered hash chain should fail verification", func(t *testing.T) {
		bundle := NewEvidenceBundle("EVENT", uuid.New())
		bundle.Events = []EventEvidence{
			{
				EventID:        uuid.New(),
				ClientRecordID: "record-001",
				SourceType:     "GW",
				SourceID:       "gateway-001",
				EventType:      "TEST",
				TsServer:       time.Now().UTC(),
				Payload:        json.RawMessage(`{}`),
				SchemaVersion:  "1.0.0",
				TrustLevel:     TrustLevelT2,
			},
		}
		bundle.HashChain = []HashChainEntry{
			{
				EventID:     uuid.New(),
				SourceID:    "gateway-001",
				SequenceNum: 1,
				EventHash:   "abc123",
				PrevHash:    "",
				ChainHash:   "computed-hash",
				TrustLevel:  TrustLevelT2,
				CreatedAt:   time.Now().UTC(),
			},
		}

		err := bundle.Finalize()
		require.NoError(t, err)

		// Tamper with HashChain
		bundle.HashChain[0].ChainHash = "tampered"
		assert.False(t, bundle.VerifyIntegrity())
	})

	t.Run("tampered signature should fail verification", func(t *testing.T) {
		bundle := NewEvidenceBundle("EVENT", uuid.New())
		bundle.Events = []EventEvidence{
			{
				EventID:        uuid.New(),
				ClientRecordID: "record-001",
				SourceType:     "GW",
				SourceID:       "gateway-001",
				EventType:      "TEST",
				TsServer:       time.Now().UTC(),
				Payload:        json.RawMessage(`{}`),
				SchemaVersion:  "1.0.0",
				TrustLevel:     TrustLevelT2,
			},
		}
		bundle.Signatures = []BatchSignature{
			{
				BatchID:            uuid.New(),
				SourceID:           "gateway-001",
				BatchHash:          "hash123",
				Signature:          "sig456",
				SignatureAlg:       "HMAC-SHA256",
				KeyID:              "key-001",
				SignedAt:           time.Now().UTC(),
				VerificationStatus: VerificationStatusVerified,
				CreatedAt:          time.Now().UTC(),
			},
		}

		err := bundle.Finalize()
		require.NoError(t, err)

		// Tamper with Signature
		bundle.Signatures[0].Signature = "tampered-signature"
		assert.False(t, bundle.VerifyIntegrity())
	})

	t.Run("non-finalized bundle should fail verification", func(t *testing.T) {
		bundle := NewEvidenceBundle("EVENT", uuid.New())
		bundle.Events = []EventEvidence{
			{
				EventID:        uuid.New(),
				ClientRecordID: "record-001",
				SourceType:     "GW",
				SourceID:       "gateway-001",
				EventType:      "TEST",
				TsServer:       time.Now().UTC(),
				Payload:        json.RawMessage(`{}`),
				SchemaVersion:  "1.0.0",
				TrustLevel:     TrustLevelT2,
			},
		}

		// Do NOT call Finalize() - BundleHash remains empty
		assert.Empty(t, bundle.BundleHash)
		assert.False(t, bundle.VerifyIntegrity())
	})
}

func TestTrustMetadata(t *testing.T) {
	eventID := uuid.New()
	chainEntryID := int64(123)
	batchID := uuid.New()
	now := time.Now().UTC()

	metadata := &TrustMetadata{
		EventID:      eventID,
		TrustLevel:   TrustLevelT3,
		ChainEntryID: &chainEntryID,
		BatchID:      &batchID,
		VerifiedAt:   &now,
		Flags:        []string{"CHAIN_VERIFIED", "SIGNATURE_VERIFIED"},
	}

	assert.Equal(t, eventID, metadata.EventID)
	assert.Equal(t, TrustLevelT3, metadata.TrustLevel)
	assert.NotNil(t, metadata.ChainEntryID)
	assert.Equal(t, chainEntryID, *metadata.ChainEntryID)
	assert.NotNil(t, metadata.BatchID)
	assert.Equal(t, batchID, *metadata.BatchID)
	assert.Len(t, metadata.Flags, 2)
}

func TestTrustVerificationResult(t *testing.T) {
	eventID := uuid.New()
	sigValid := true

	result := &TrustVerificationResult{
		EventID:        eventID,
		TrustLevel:     TrustLevelT3,
		ChainValid:     true,
		SignatureValid: &sigValid,
		Errors:         nil,
		VerifiedAt:     time.Now().UTC(),
	}

	assert.Equal(t, eventID, result.EventID)
	assert.Equal(t, TrustLevelT3, result.TrustLevel)
	assert.True(t, result.ChainValid)
	assert.NotNil(t, result.SignatureValid)
	assert.True(t, *result.SignatureValid)
	assert.Empty(t, result.Errors)
}
