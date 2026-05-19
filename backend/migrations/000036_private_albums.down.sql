DROP INDEX IF EXISTS idx_private_album_views_viewer;
DROP INDEX IF EXISTS idx_private_album_views_album_viewed;
DROP TABLE IF EXISTS private_album_views;

DROP INDEX IF EXISTS idx_private_album_grants_album_status;
DROP INDEX IF EXISTS idx_private_album_grants_grantee;
DROP INDEX IF EXISTS uq_private_album_grants_open;
DROP TABLE IF EXISTS private_album_grants;

DROP INDEX IF EXISTS idx_private_album_photos_position;
DROP INDEX IF EXISTS idx_private_album_photos_upload;
DROP TABLE IF EXISTS private_album_photos;

DROP INDEX IF EXISTS idx_private_albums_owner;
DROP TABLE IF EXISTS private_albums;
