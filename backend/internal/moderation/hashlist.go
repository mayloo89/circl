package moderation

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HashEntry is a single row from the image_block_hashes table.
type HashEntry struct {
	Hash   string
	Source string // "local" | "stopncii" | "photodna" | ...
	Reason string
}

// HashStore is the persistence interface HashList needs. The pg implementation
// is in this file; tests fake the interface directly.
type HashStore interface {
	Lookup(ctx context.Context, hash string) (*HashEntry, error)
	// Add inserts a hash (admin operation). Returns nil if the hash already
	// exists with the same source so the caller treats it as idempotent.
	Add(ctx context.Context, e HashEntry, addedBy string) error
}

// ErrHashNotFound is returned by Lookup when the hash is not in the block
// list. Callers translate this to "no decision" — not a rejection.
var ErrHashNotFound = errors.New("moderation: hash not in block list")

// HashList rejects uploads whose SHA-256 is present in the image_block_hashes
// table. Seeded locally today; the same table is the drop-point for the
// StopNCII feed when that integration lands.
type HashList struct {
	store HashStore
}

// NewHashList returns a HashList backed by the given store.
func NewHashList(store HashStore) *HashList {
	return &HashList{store: store}
}

// Name returns the detector name.
func (h *HashList) Name() string { return "hashlist" }

// Check looks the hash up in the block list. A hit always rejects; a miss
// allows. Store errors propagate up — the orchestrator decides whether to
// fail open or hard. The reason returned to the user is intentionally
// non-specific ("matched a known-bad content fingerprint") so a uploader
// probing for which hashes are blocked gets the same wording every time.
func (h *HashList) Check(ctx context.Context, in Input) (Decision, error) {
	if in.Hash == "" {
		return Decision{}, ErrInputMissing
	}
	entry, err := h.store.Lookup(ctx, in.Hash)
	if err != nil {
		if errors.Is(err, ErrHashNotFound) {
			return Allow(), nil
		}
		return Decision{}, err
	}
	// The internal reason carries the source for the audit trail; the
	// user-facing reason stays generic.
	reason := "matched a known-bad content fingerprint"
	if entry.Reason != "" {
		reason = entry.Reason
	}
	return Reject(CodeHashMatch, reason, h.Name()+":"+entry.Source), nil
}

// pgHashStore is the Postgres-backed HashStore.
type pgHashStore struct {
	pool *pgxpool.Pool
}

// NewPgHashStore returns a HashStore backed by pgx.
func NewPgHashStore(pool *pgxpool.Pool) HashStore {
	return &pgHashStore{pool: pool}
}

func (s *pgHashStore) Lookup(ctx context.Context, hash string) (*HashEntry, error) {
	var e HashEntry
	err := s.pool.QueryRow(ctx,
		`SELECT hash, source, reason FROM image_block_hashes WHERE hash = $1`,
		hash,
	).Scan(&e.Hash, &e.Source, &e.Reason)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrHashNotFound
		}
		return nil, fmt.Errorf("lookup hash: %w", err)
	}
	return &e, nil
}

func (s *pgHashStore) Add(ctx context.Context, e HashEntry, addedBy string) error {
	var addedByArg any
	if addedBy != "" {
		addedByArg = addedBy
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO image_block_hashes (hash, source, reason, added_by)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (hash) DO NOTHING`,
		e.Hash, e.Source, e.Reason, addedByArg,
	)
	if err != nil {
		return fmt.Errorf("add hash: %w", err)
	}
	return nil
}
