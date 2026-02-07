package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/execution-hub/execution-hub/internal/domain/trust"
)

// TrustRepository implements trust.Repository.
type TrustRepository struct {
	pool *pgxpool.Pool
}

func NewTrustRepository(pool *pgxpool.Pool) *TrustRepository {
	return &TrustRepository{pool: pool}
}

func (r *TrustRepository) InsertEvent(ctx context.Context, event *trust.EventEvidence) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO events
		(event_id, client_record_id, source_type, source_id, ts_device, ts_gateway, ts_server, key, event_type, payload, schema_version, trust_level)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (event_id) DO UPDATE
		SET client_record_id=EXCLUDED.client_record_id,
			source_type=EXCLUDED.source_type,
			source_id=EXCLUDED.source_id,
			ts_device=EXCLUDED.ts_device,
			ts_gateway=EXCLUDED.ts_gateway,
			ts_server=EXCLUDED.ts_server,
			key=EXCLUDED.key,
			event_type=EXCLUDED.event_type,
			payload=EXCLUDED.payload,
			schema_version=EXCLUDED.schema_version,
			trust_level=EXCLUDED.trust_level
	`, event.EventID, event.ClientRecordID, event.SourceType, event.SourceID, event.TsDevice, event.TsGateway, event.TsServer, event.Key, event.EventType, event.Payload, event.SchemaVersion, event.TrustLevel)
	return err
}

func (r *TrustRepository) GetEvent(ctx context.Context, eventID uuid.UUID) (*trust.EventEvidence, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT event_id, client_record_id, source_type, source_id, ts_device, ts_gateway, ts_server, key, event_type, payload, schema_version, trust_level
		FROM events WHERE event_id=$1
	`, eventID)
	return scanEvent(row)
}

func (r *TrustRepository) GetEvents(ctx context.Context, eventIDs []uuid.UUID) ([]trust.EventEvidence, error) {
	if len(eventIDs) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT event_id, client_record_id, source_type, source_id, ts_device, ts_gateway, ts_server, key, event_type, payload, schema_version, trust_level
		FROM events WHERE event_id = ANY($1)
	`, eventIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []trust.EventEvidence
	for rows.Next() {
		ev, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			out = append(out, *ev)
		}
	}
	return out, rows.Err()
}

func (r *TrustRepository) InsertHashChainEntry(ctx context.Context, entry *trust.HashChainEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO trust_hash_chain_entries
		(event_id, source_id, sequence_num, event_hash, prev_hash, chain_hash, trust_level, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, entry.EventID, entry.SourceID, entry.SequenceNum, entry.EventHash, entry.PrevHash, entry.ChainHash, entry.TrustLevel, entry.CreatedAt)
	return err
}

func (r *TrustRepository) GetHashChainEntry(ctx context.Context, eventID uuid.UUID) (*trust.HashChainEntry, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, event_id, source_id, sequence_num, event_hash, prev_hash, chain_hash, trust_level, created_at
		FROM trust_hash_chain_entries WHERE event_id=$1
	`, eventID)
	return scanHashChain(row)
}

func (r *TrustRepository) GetLatestChainEntry(ctx context.Context, sourceID string) (*trust.HashChainEntry, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, event_id, source_id, sequence_num, event_hash, prev_hash, chain_hash, trust_level, created_at
		FROM trust_hash_chain_entries WHERE source_id=$1 ORDER BY sequence_num DESC LIMIT 1
	`, sourceID)
	return scanHashChain(row)
}

func (r *TrustRepository) GetChainEntriesForSource(ctx context.Context, sourceID string, fromSeq, toSeq int64) ([]*trust.HashChainEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, source_id, sequence_num, event_hash, prev_hash, chain_hash, trust_level, created_at
		FROM trust_hash_chain_entries WHERE source_id=$1 AND sequence_num >= $2 AND sequence_num <= $3 ORDER BY sequence_num ASC
	`, sourceID, fromSeq, toSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*trust.HashChainEntry
	for rows.Next() {
		entry, err := scanHashChain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *TrustRepository) GetChainEntriesForEvents(ctx context.Context, eventIDs []uuid.UUID) ([]*trust.HashChainEntry, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_id, source_id, sequence_num, event_hash, prev_hash, chain_hash, trust_level, created_at
		FROM trust_hash_chain_entries WHERE event_id = ANY($1)
	`, eventIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*trust.HashChainEntry
	for rows.Next() {
		entry, err := scanHashChain(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *TrustRepository) InsertBatchSignature(ctx context.Context, sig *trust.BatchSignature) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO trust_batch_signatures
		(batch_id, source_id, event_ids, batch_hash, signature, signature_alg, key_id, signed_at, verified_at, verification_status, verification_error, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, sig.BatchID, sig.SourceID, sig.EventIDs, sig.BatchHash, sig.Signature, sig.SignatureAlg, sig.KeyID, sig.SignedAt, sig.VerifiedAt, sig.VerificationStatus, sig.VerificationError, sig.CreatedAt)
	return err
}

func (r *TrustRepository) GetBatchSignature(ctx context.Context, batchID uuid.UUID) (*trust.BatchSignature, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, batch_id, source_id, event_ids, batch_hash, signature, signature_alg, key_id, signed_at, verified_at, verification_status, verification_error, created_at
		FROM trust_batch_signatures WHERE batch_id=$1
	`, batchID)
	return scanBatchSignature(row)
}

func (r *TrustRepository) GetBatchSignaturesBySource(ctx context.Context, sourceID string, limit int) ([]*trust.BatchSignature, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, batch_id, source_id, event_ids, batch_hash, signature, signature_alg, key_id, signed_at, verified_at, verification_status, verification_error, created_at
		FROM trust_batch_signatures WHERE source_id=$1 ORDER BY signed_at DESC LIMIT $2
	`, sourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*trust.BatchSignature
	for rows.Next() {
		sig, err := scanBatchSignature(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sig)
	}
	return out, rows.Err()
}

func (r *TrustRepository) GetBatchSignatureForEvent(ctx context.Context, eventID uuid.UUID) (*trust.BatchSignature, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, batch_id, source_id, event_ids, batch_hash, signature, signature_alg, key_id, signed_at, verified_at, verification_status, verification_error, created_at
		FROM trust_batch_signatures WHERE $1 = ANY(event_ids) ORDER BY signed_at DESC LIMIT 1
	`, eventID)
	return scanBatchSignature(row)
}

func (r *TrustRepository) UpdateBatchSignatureStatus(ctx context.Context, batchID uuid.UUID, status trust.VerificationStatus, errMsg *string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE trust_batch_signatures SET verification_status=$1, verification_error=$2, verified_at=CASE WHEN $1='VERIFIED' THEN NOW() ELSE verified_at END WHERE batch_id=$3
	`, status, errMsg, batchID)
	return err
}

func (r *TrustRepository) GetTrustMetadata(ctx context.Context, eventID uuid.UUID) (*trust.TrustMetadata, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT event_id, trust_level, chain_entry_id, batch_id, verified_at, flags, updated_at
		FROM trust_metadata WHERE event_id=$1
	`, eventID)
	return scanTrustMetadata(row)
}

func (r *TrustRepository) BatchGetTrustMetadata(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]*trust.TrustMetadata, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT event_id, trust_level, chain_entry_id, batch_id, verified_at, flags, updated_at
		FROM trust_metadata WHERE event_id = ANY($1)
	`, eventIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uuid.UUID]*trust.TrustMetadata)
	for rows.Next() {
		meta, err := scanTrustMetadata(rows)
		if err != nil {
			return nil, err
		}
		out[meta.EventID] = meta
	}
	return out, rows.Err()
}

func (r *TrustRepository) UpdateEventTrustLevel(ctx context.Context, eventID uuid.UUID, level trust.TrustLevel) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO trust_metadata (event_id, trust_level, updated_at)
		VALUES ($1,$2,NOW())
		ON CONFLICT (event_id) DO UPDATE SET trust_level=$2, updated_at=NOW()
	`, eventID, level)
	if err != nil {
		return err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE events SET trust_level=$1 WHERE event_id=$2`, level, eventID)
	return nil
}

func (r *TrustRepository) GetEvidenceBundleData(ctx context.Context, bundleType string, subjectID uuid.UUID) (*trust.EvidenceBundleData, error) {
	data := &trust.EvidenceBundleData{}

	switch bundleType {
	case "ACTION":
		// Build Action evidence from action + step evidence.
		row := r.pool.QueryRow(ctx, `
			SELECT action_id, rule_id, rule_version, evaluation_id, action_type, action_config, status, created_at
			FROM actions WHERE action_id=$1
		`, subjectID)
		var action trust.ActionEvidence
		if err := row.Scan(&action.ActionID, &action.RuleID, &action.RuleVersion, &action.EvaluationID, &action.ActionType, &action.ActionConfig, &action.Status, &action.CreatedAt); err != nil {
			if err == pgx.ErrNoRows {
				return nil, trust.ErrEventNotFound
			}
			return nil, err
		}
		// Attach step evidence if exists.
		row = r.pool.QueryRow(ctx, `SELECT evidence FROM task_steps WHERE action_id=$1`, subjectID)
		var evidence json.RawMessage
		if err := row.Scan(&evidence); err == nil {
			action.Evidence = evidence
		}
		data.Actions = []trust.ActionEvidence{action}

		// Include rule/evaluation evidence when present.
		if action.RuleID != uuid.Nil {
			row = r.pool.QueryRow(ctx, `
				SELECT rule_id, version, name, rule_type, config, effective_from, effective_until
				FROM rules WHERE rule_id=$1 AND version=$2
			`, action.RuleID, action.RuleVersion)
			var re trust.RuleEvidence
			if err := row.Scan(&re.RuleID, &re.Version, &re.Name, &re.RuleType, &re.Config, &re.EffectiveFrom, &re.EffectiveUntil); err == nil {
				data.Rules = append(data.Rules, re)
			}
		}
		var evalEventIDs []uuid.UUID
		if action.EvaluationID != uuid.Nil {
			row = r.pool.QueryRow(ctx, `
				SELECT evaluation_id, rule_id, rule_version, matched, evaluated_at, evidence, event_ids
				FROM rule_evaluations WHERE evaluation_id=$1
			`, action.EvaluationID)
			var ev trust.EvaluationEvidence
			var evidenceRaw json.RawMessage
			if err := row.Scan(&ev.EvaluationID, &ev.RuleID, &ev.RuleVersion, &ev.Matched, &ev.EvaluatedAt, &evidenceRaw, &evalEventIDs); err == nil {
				ev.Evidence = evidenceRaw
				data.Evaluations = append(data.Evaluations, ev)
			}
		}
		if len(evalEventIDs) > 0 {
			events, _ := r.GetEvents(ctx, evalEventIDs)
			data.Events = append(data.Events, events...)
			chainEntries, _ := r.GetChainEntriesForEvents(ctx, evalEventIDs)
			data.HashChain = append(data.HashChain, chainEntries...)
			sigMap := map[uuid.UUID]struct{}{}
			for _, eid := range evalEventIDs {
				sig, _ := r.GetBatchSignatureForEvent(ctx, eid)
				if sig != nil {
					if _, ok := sigMap[sig.BatchID]; !ok {
						sigMap[sig.BatchID] = struct{}{}
						data.Signatures = append(data.Signatures, sig)
					}
				}
			}
		}
		return data, nil
	case "TASK":
		// Ensure task exists.
		row := r.pool.QueryRow(ctx, `SELECT task_id FROM tasks WHERE task_id=$1`, subjectID)
		var taskID uuid.UUID
		if err := row.Scan(&taskID); err != nil {
			if err == pgx.ErrNoRows {
				return nil, trust.ErrEventNotFound
			}
			return nil, err
		}

		// Collect actions for the task via steps.
		rows, err := r.pool.Query(ctx, `
			SELECT a.action_id, a.rule_id, a.rule_version, a.evaluation_id, a.action_type, a.action_config, a.status, a.created_at, s.evidence
			FROM actions a
			JOIN task_steps s ON s.action_id = a.action_id
			WHERE s.task_id=$1
			ORDER BY s.created_at ASC
		`, subjectID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		type ruleKey struct {
			id      uuid.UUID
			version int
		}
		ruleKeys := make(map[ruleKey]struct{})
		evalIDs := make([]uuid.UUID, 0)
		evalSet := make(map[uuid.UUID]struct{})

		for rows.Next() {
			var action trust.ActionEvidence
			var evidence json.RawMessage
			if err := rows.Scan(&action.ActionID, &action.RuleID, &action.RuleVersion, &action.EvaluationID, &action.ActionType, &action.ActionConfig, &action.Status, &action.CreatedAt, &evidence); err != nil {
				return nil, err
			}
			action.Evidence = evidence
			data.Actions = append(data.Actions, action)

			if action.RuleID != uuid.Nil {
				ruleKeys[ruleKey{id: action.RuleID, version: action.RuleVersion}] = struct{}{}
			}
			if action.EvaluationID != uuid.Nil {
				if _, ok := evalSet[action.EvaluationID]; !ok {
					evalSet[action.EvaluationID] = struct{}{}
					evalIDs = append(evalIDs, action.EvaluationID)
				}
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		// Load rule evidence.
		for key := range ruleKeys {
			row = r.pool.QueryRow(ctx, `
				SELECT rule_id, version, name, rule_type, config, effective_from, effective_until
				FROM rules WHERE rule_id=$1 AND version=$2
			`, key.id, key.version)
			var re trust.RuleEvidence
			if err := row.Scan(&re.RuleID, &re.Version, &re.Name, &re.RuleType, &re.Config, &re.EffectiveFrom, &re.EffectiveUntil); err == nil {
				data.Rules = append(data.Rules, re)
			}
		}

		// Load evaluations and events.
		eventSet := make(map[uuid.UUID]struct{})
		eventIDs := make([]uuid.UUID, 0)
		if len(evalIDs) > 0 {
			rows, err = r.pool.Query(ctx, `
				SELECT evaluation_id, rule_id, rule_version, matched, evaluated_at, evidence, event_ids
				FROM rule_evaluations WHERE evaluation_id = ANY($1)
			`, evalIDs)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			for rows.Next() {
				var ev trust.EvaluationEvidence
				var evidenceRaw json.RawMessage
				var evalEventIDs []uuid.UUID
				if err := rows.Scan(&ev.EvaluationID, &ev.RuleID, &ev.RuleVersion, &ev.Matched, &ev.EvaluatedAt, &evidenceRaw, &evalEventIDs); err != nil {
					return nil, err
				}
				ev.Evidence = evidenceRaw
				data.Evaluations = append(data.Evaluations, ev)
				for _, eid := range evalEventIDs {
					if _, ok := eventSet[eid]; !ok {
						eventSet[eid] = struct{}{}
						eventIDs = append(eventIDs, eid)
					}
				}
			}
			if err := rows.Err(); err != nil {
				return nil, err
			}
		}

		if len(eventIDs) > 0 {
			events, _ := r.GetEvents(ctx, eventIDs)
			data.Events = append(data.Events, events...)
			chainEntries, _ := r.GetChainEntriesForEvents(ctx, eventIDs)
			data.HashChain = append(data.HashChain, chainEntries...)
			sigMap := map[uuid.UUID]struct{}{}
			for _, eid := range eventIDs {
				sig, _ := r.GetBatchSignatureForEvent(ctx, eid)
				if sig != nil {
					if _, ok := sigMap[sig.BatchID]; !ok {
						sigMap[sig.BatchID] = struct{}{}
						data.Signatures = append(data.Signatures, sig)
					}
				}
			}
		}

		return data, nil
	case "BATCH":
		row := r.pool.QueryRow(ctx, `
			SELECT id, batch_id, source_id, event_ids, batch_hash, signature, signature_alg, key_id, signed_at, verified_at, verification_status, verification_error, created_at
			FROM trust_batch_signatures WHERE batch_id=$1
		`, subjectID)
		sig, err := scanBatchSignature(row)
		if err != nil {
			return nil, err
		}
		if sig != nil {
			data.Signatures = []*trust.BatchSignature{sig}
			if len(sig.EventIDs) > 0 {
				events, _ := r.GetEvents(ctx, sig.EventIDs)
				data.Events = append(data.Events, events...)
				chainEntries, _ := r.GetChainEntriesForEvents(ctx, sig.EventIDs)
				data.HashChain = append(data.HashChain, chainEntries...)
			}
		}
		return data, nil
	case "EVENT":
		event, err := r.GetEvent(ctx, subjectID)
		if err != nil {
			return nil, err
		}
		if event == nil {
			return nil, trust.ErrEventNotFound
		}
		data.Events = []trust.EventEvidence{*event}
		chain, _ := r.GetHashChainEntry(ctx, subjectID)
		if chain != nil {
			data.HashChain = []*trust.HashChainEntry{chain}
		}
		sig, _ := r.GetBatchSignatureForEvent(ctx, subjectID)
		if sig != nil {
			data.Signatures = []*trust.BatchSignature{sig}
		}
		return data, nil
	default:
		return nil, trust.ErrInvalidBundleType
	}
}

func scanEvent(row pgx.Row) (*trust.EventEvidence, error) {
	var ev trust.EventEvidence
	var payload json.RawMessage
	if err := row.Scan(&ev.EventID, &ev.ClientRecordID, &ev.SourceType, &ev.SourceID, &ev.TsDevice, &ev.TsGateway, &ev.TsServer, &ev.Key, &ev.EventType, &payload, &ev.SchemaVersion, &ev.TrustLevel); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if len(payload) > 0 {
		ev.Payload = payload
	}
	return &ev, nil
}

func scanHashChain(row pgx.Row) (*trust.HashChainEntry, error) {
	var e trust.HashChainEntry
	if err := row.Scan(&e.ID, &e.EventID, &e.SourceID, &e.SequenceNum, &e.EventHash, &e.PrevHash, &e.ChainHash, &e.TrustLevel, &e.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

func scanBatchSignature(row pgx.Row) (*trust.BatchSignature, error) {
	var s trust.BatchSignature
	if err := row.Scan(&s.ID, &s.BatchID, &s.SourceID, &s.EventIDs, &s.BatchHash, &s.Signature, &s.SignatureAlg, &s.KeyID, &s.SignedAt, &s.VerifiedAt, &s.VerificationStatus, &s.VerificationError, &s.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func scanTrustMetadata(row pgx.Row) (*trust.TrustMetadata, error) {
	var m trust.TrustMetadata
	var updatedAt time.Time
	if err := row.Scan(&m.EventID, &m.TrustLevel, &m.ChainEntryID, &m.BatchID, &m.VerifiedAt, &m.Flags, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_ = updatedAt
	return &m, nil
}
