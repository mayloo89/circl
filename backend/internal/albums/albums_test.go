package albums_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/mayloo89/circl/backend/internal/albums"
	"github.com/mayloo89/circl/backend/internal/storage"
	"github.com/mayloo89/circl/backend/internal/uploads"
)

// fakeStore is a hand-rolled in-memory implementation of albums.Store with
// just enough behaviour to drive the service. We don't aim for behavioural
// fidelity with the pg-backed store — only enough to verify the
// permission and lifecycle logic the service layer enforces.
type fakeStore struct {
	albums map[string]*albums.Album
	photos map[string]*albums.Photo // key = albumID:uploadID
	grants map[string]*albums.Grant
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		albums: map[string]*albums.Album{},
		photos: map[string]*albums.Photo{},
		grants: map[string]*albums.Grant{},
	}
}

func (s *fakeStore) CreateAlbum(_ context.Context, ownerID, name, description string) (*albums.Album, error) {
	a := &albums.Album{
		ID: "a-" + name, OwnerID: ownerID, Name: name, Description: description,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.albums[a.ID] = a
	return a, nil
}

func (s *fakeStore) GetAlbum(_ context.Context, id string) (*albums.Album, error) {
	a, ok := s.albums[id]
	if !ok {
		return nil, albums.ErrNotFound
	}
	c := *a
	return &c, nil
}

func (s *fakeStore) ListAlbumsByOwner(_ context.Context, ownerID string) ([]albums.Album, error) {
	out := []albums.Album{}
	for _, a := range s.albums {
		if a.OwnerID == ownerID {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (s *fakeStore) ListAlbumsSharedWith(_ context.Context, granteeID string) ([]albums.Album, error) {
	out := []albums.Album{}
	for _, g := range s.grants {
		if g.GranteeID == granteeID && g.Status == albums.GrantActive {
			if a, ok := s.albums[g.AlbumID]; ok {
				out = append(out, *a)
			}
		}
	}
	return out, nil
}

func (s *fakeStore) UpdateAlbum(_ context.Context, id, name, description string) (*albums.Album, error) {
	a, ok := s.albums[id]
	if !ok {
		return nil, albums.ErrNotFound
	}
	a.Name = name
	a.Description = description
	a.UpdatedAt = time.Now()
	c := *a
	return &c, nil
}

func (s *fakeStore) DeleteAlbum(_ context.Context, id string) error {
	if _, ok := s.albums[id]; !ok {
		return albums.ErrNotFound
	}
	delete(s.albums, id)
	return nil
}

func (s *fakeStore) AddPhoto(_ context.Context, albumID, uploadID string, position int) error {
	s.photos[albumID+":"+uploadID] = &albums.Photo{
		AlbumID: albumID, UploadID: uploadID, Position: position, AddedAt: time.Now(),
	}
	if a, ok := s.albums[albumID]; ok {
		a.PhotoCount++
	}
	return nil
}

func (s *fakeStore) RemovePhoto(_ context.Context, albumID, uploadID string) error {
	delete(s.photos, albumID+":"+uploadID)
	if a, ok := s.albums[albumID]; ok && a.PhotoCount > 0 {
		a.PhotoCount--
	}
	return nil
}

func (s *fakeStore) ListPhotos(_ context.Context, albumID string) ([]albums.Photo, error) {
	out := []albums.Photo{}
	for _, p := range s.photos {
		if p.AlbumID == albumID {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (s *fakeStore) GetPhoto(_ context.Context, albumID, uploadID string) (*albums.Photo, error) {
	p, ok := s.photos[albumID+":"+uploadID]
	if !ok {
		return nil, albums.ErrNotFound
	}
	c := *p
	return &c, nil
}

func (s *fakeStore) CreateGrant(_ context.Context, albumID, granterID, granteeID, status, source string, expiresAt *time.Time) (*albums.Grant, error) {
	g := &albums.Grant{
		ID: "g-" + albumID + "-" + granteeID, AlbumID: albumID,
		GranterID: granterID, GranteeID: granteeID,
		Status: status, Source: source, RequestedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
	if status == albums.GrantActive {
		now := time.Now()
		g.GrantedAt = &now
	}
	s.grants[g.ID] = g
	return g, nil
}

func (s *fakeStore) ExpireAlbumGrants(_ context.Context) error { return nil }

func (s *fakeStore) GetGrant(_ context.Context, id string) (*albums.Grant, error) {
	g, ok := s.grants[id]
	if !ok {
		return nil, albums.ErrNotFound
	}
	c := *g
	return &c, nil
}

func (s *fakeStore) UpdateGrantStatus(_ context.Context, id, status string) (*albums.Grant, error) {
	g, ok := s.grants[id]
	if !ok {
		return nil, albums.ErrNotFound
	}
	g.Status = status
	now := time.Now()
	if status == albums.GrantActive && g.GrantedAt == nil {
		g.GrantedAt = &now
	}
	if status == albums.GrantRevoked && g.RevokedAt == nil {
		g.RevokedAt = &now
	}
	c := *g
	return &c, nil
}

func (s *fakeStore) ListGrantsByAlbum(_ context.Context, albumID string) ([]albums.Grant, error) {
	out := []albums.Grant{}
	for _, g := range s.grants {
		if g.AlbumID == albumID {
			out = append(out, *g)
		}
	}
	return out, nil
}

func (s *fakeStore) FindOpenGrant(_ context.Context, albumID, granteeID string) (*albums.Grant, error) {
	for _, g := range s.grants {
		if g.AlbumID == albumID && g.GranteeID == granteeID &&
			(g.Status == albums.GrantPending || g.Status == albums.GrantActive) {
			c := *g
			return &c, nil
		}
	}
	return nil, albums.ErrNotFound
}

func (s *fakeStore) HasActiveGrant(_ context.Context, albumID, viewerID string) (bool, error) {
	for _, g := range s.grants {
		if g.AlbumID == albumID && g.GranteeID == viewerID && g.Status == albums.GrantActive {
			return true, nil
		}
	}
	return false, nil
}


type fakeMedia struct {
	owner            string
	category         string
	moderationStatus string // defaults to "approved"
}

func (f fakeMedia) GetForOwner(_ context.Context, uploadID, ownerID string) (*uploads.Upload, error) {
	if ownerID != f.owner {
		return nil, errors.New("forbidden")
	}
	status := f.moderationStatus
	if status == "" {
		status = "approved"
	}
	return &uploads.Upload{ID: uploadID, UserID: ownerID, Category: f.category, ModerationStatus: status, StorageKey: "k/" + uploadID, ContentType: "image/jpeg"}, nil
}

func newSvc(t *testing.T, ownerID, uploadCategory string) (*albums.Service, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	svc := albums.NewService(store, nil, fakeMedia{owner: ownerID, category: uploadCategory}, zerolog.Nop())
	return svc, store
}

// --- Tests ---

func TestService_CreateAndGet_OwnerSeesRoleOwner(t *testing.T) {
	svc, _ := newSvc(t, "u-1", "")
	a, err := svc.CreateAlbum(t.Context(), "u-1", "After-hours", "draft")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.GetAlbum(t.Context(), "u-1", a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Role != "owner" {
		t.Errorf("role = %q, want owner", got.Role)
	}
}

func TestService_GetAlbum_NonGranteeForbidden(t *testing.T) {
	svc, _ := newSvc(t, "u-1", "")
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")
	_, err := svc.GetAlbum(t.Context(), "u-2", a.ID)
	if !errors.Is(err, albums.ErrForbidden) {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

func TestService_Invite_GrantsImmediateAccess(t *testing.T) {
	svc, _ := newSvc(t, "u-1", "")
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")

	g, err := svc.InviteUser(t.Context(), "u-1", a.ID, "u-2", nil)
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if g.Status != albums.GrantActive || g.Source != albums.SourceInvite {
		t.Errorf("status/source = %q/%q, want active/invite", g.Status, g.Source)
	}

	// Grantee can fetch the album immediately — no accept step needed.
	got, err := svc.GetAlbum(t.Context(), "u-2", a.ID)
	if err != nil {
		t.Fatalf("get after invite: %v", err)
	}
	if got.Role != "viewer" {
		t.Errorf("role = %q, want viewer", got.Role)
	}
}

func TestService_Invite_DuplicateOpenGrantRejected(t *testing.T) {
	svc, _ := newSvc(t, "u-1", "")
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")
	if _, err := svc.InviteUser(t.Context(), "u-1", a.ID, "u-2", nil); err != nil {
		t.Fatalf("first invite: %v", err)
	}
	_, err := svc.InviteUser(t.Context(), "u-1", a.ID, "u-2", nil)
	if !errors.Is(err, albums.ErrGrantExists) {
		t.Errorf("err = %v, want ErrGrantExists", err)
	}
}

func TestService_RequestAccess_AcceptedByOwnerOnly(t *testing.T) {
	svc, _ := newSvc(t, "u-1", "")
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")

	g, err := svc.RequestAccess(t.Context(), "u-2", a.ID)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	// Grantee cannot accept their own request — the owner must.
	if _, err := svc.AcceptGrant(t.Context(), "u-2", g.ID); !errors.Is(err, albums.ErrForbidden) {
		t.Errorf("grantee accept: err = %v, want forbidden", err)
	}
	// Owner accepts.
	if _, err := svc.AcceptGrant(t.Context(), "u-1", g.ID); err != nil {
		t.Fatalf("owner accept: %v", err)
	}
}

func TestService_Revoke_EitherSide(t *testing.T) {
	svc, store := newSvc(t, "u-1", "")
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")
	g, _ := svc.InviteUser(t.Context(), "u-1", a.ID, "u-2", nil)

	// Grantee revokes their own access.
	if _, err := svc.RevokeGrant(t.Context(), "u-2", g.ID); err != nil {
		t.Fatalf("revoke by grantee: %v", err)
	}
	if got := store.grants[g.ID]; got.Status != albums.GrantRevoked {
		t.Errorf("status = %q, want revoked", got.Status)
	}

	// Stranger cannot revoke.
	g2, _ := svc.InviteUser(t.Context(), "u-1", a.ID, "u-2", nil)
	if _, err := svc.RevokeGrant(t.Context(), "u-3", g2.ID); !errors.Is(err, albums.ErrForbidden) {
		t.Errorf("err = %v, want forbidden", err)
	}
}

func TestService_AddPhoto_RejectsWrongCategory(t *testing.T) {
	// Media resolver returns a gallery-category upload — service must reject
	// because the album-private feature only accepts its own category to
	// keep public-gallery photos from leaking into private albums.
	svc, _ := newSvc(t, "u-1", string(storage.CategoryGallery))
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")
	err := svc.AddPhoto(t.Context(), "u-1", a.ID, "up-1")
	if !errors.Is(err, albums.ErrInvalidRequest) {
		t.Errorf("err = %v, want invalid request", err)
	}
}

func TestService_AddPhoto_AcceptsAlbumPrivateCategory(t *testing.T) {
	svc, _ := newSvc(t, "u-1", string(storage.CategoryAlbumPrivate))
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")
	if err := svc.AddPhoto(t.Context(), "u-1", a.ID, "up-1"); err != nil {
		t.Fatalf("add photo: %v", err)
	}
}

func TestService_AddPhoto_RejectsUnmoderatedUpload(t *testing.T) {
	// A still-pending (not-yet-scanned) upload must be blocked from joining an
	// album — not just rejected ones.
	store := newFakeStore()
	svc := albums.NewService(store, nil, fakeMedia{
		owner:            "u-1",
		category:         string(storage.CategoryAlbumPrivate),
		moderationStatus: "pending",
	}, zerolog.Nop())
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")
	err := svc.AddPhoto(t.Context(), "u-1", a.ID, "up-1")
	if !errors.Is(err, albums.ErrUploadNotApproved) {
		t.Errorf("err = %v, want ErrUploadNotApproved", err)
	}
}

func TestService_ShareInChat_ReusesExistingActiveGrant(t *testing.T) {
	svc, store := newSvc(t, "u-1", string(storage.CategoryAlbumPrivate))
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")

	// Invite creates an active grant; ShareInChat should reuse it rather
	// than create a duplicate.
	g, _ := svc.InviteUser(t.Context(), "u-1", a.ID, "u-2", nil)
	_, _, err := svc.ShareInChat(t.Context(), "u-1", a.ID, "u-2", nil)
	if err != nil {
		t.Fatalf("share in chat: %v", err)
	}
	if n := len(store.grants); n != 1 {
		t.Errorf("grants count = %d, want 1 (no duplicate created)", n)
	}
	if got := store.grants[g.ID]; got.Status != albums.GrantActive {
		t.Errorf("status = %q, want active", got.Status)
	}
}

func TestService_ShareInChat_SelfGrantRejected(t *testing.T) {
	svc, _ := newSvc(t, "u-1", "")
	a, _ := svc.CreateAlbum(t.Context(), "u-1", "x", "")
	_, _, err := svc.ShareInChat(t.Context(), "u-1", a.ID, "u-1", nil)
	if !errors.Is(err, albums.ErrSelfGrant) {
		t.Errorf("err = %v, want ErrSelfGrant", err)
	}
}
