-- Expanded report categories + priority field.
-- Two new reasons back the safety page: NCII (Cloudflare CSAM / StopNCII scope)
-- and digital gender violence (Argentine Ley 27.736, "Ley Olimpia"). 'csam'
-- is added even though direct user reporting is not the primary discovery
-- channel — vendor scanning is — because admin still needs a category to
-- triage out-of-band reports against.
ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_reason_check;
ALTER TABLE reports ADD CONSTRAINT reports_reason_check
    CHECK (reason IN (
        'harassment',
        'spam',
        'inappropriate_content',
        'fake_profile',
        'non_consensual_intimate_images',
        'digital_gender_violence',
        'csam',
        'other'
    ));

ALTER TABLE reports
    ADD COLUMN IF NOT EXISTS priority TEXT NOT NULL DEFAULT 'normal'
    CHECK (priority IN ('normal', 'high', 'critical'));

-- NCII and CSAM are routed to the critical queue; gender-violence reports
-- get the high queue so admin sees them above the noise.
UPDATE reports SET priority = 'critical' WHERE reason IN ('non_consensual_intimate_images', 'csam');
UPDATE reports SET priority = 'high' WHERE reason = 'digital_gender_violence';

CREATE INDEX IF NOT EXISTS idx_reports_priority_status
    ON reports(priority, status, created_at DESC);

-- Registration age-attestation audit.
-- One row per successful registration; preserved across user hard-delete so
-- the evidence trail survives account purge. user_id is nullable for that
-- reason — we keep the IP/UA/timestamp even after the row it referenced is
-- gone.
CREATE TABLE age_verification_audit (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        REFERENCES users(id) ON DELETE SET NULL,
    user_email      TEXT        NOT NULL,
    attested_age    INT         NOT NULL,
    ip              INET,
    user_agent      TEXT        NOT NULL DEFAULT '',
    date_of_birth   DATE,
    policy_version  TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_age_audit_user ON age_verification_audit(user_id);
CREATE INDEX idx_age_audit_created_at ON age_verification_audit(created_at DESC);

-- Appeals — one row per suspension/ban that admin issued. The token is
-- emailed to the user; it identifies the row when the (locked-out) user
-- visits /appeal/{token}. status='open' means we sent the email but the
-- user has not submitted yet; 'submitted' awaits admin review; 'approved'
-- means the action was reversed; 'denied' means it stands.
CREATE TABLE appeals (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    suspension_id    UUID        NOT NULL REFERENCES suspensions(id) ON DELETE CASCADE,
    token_hash       TEXT        NOT NULL UNIQUE,
    status           TEXT        NOT NULL DEFAULT 'open'
                     CHECK (status IN ('open', 'submitted', 'approved', 'denied', 'expired')),
    body             TEXT        NOT NULL DEFAULT '',
    expires_at       TIMESTAMPTZ NOT NULL,
    submitted_at     TIMESTAMPTZ,
    resolved_at      TIMESTAMPTZ,
    resolved_by      UUID        REFERENCES users(id) ON DELETE SET NULL,
    resolution_note  TEXT        NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_appeals_user ON appeals(user_id);
CREATE INDEX idx_appeals_status ON appeals(status, created_at DESC);
CREATE UNIQUE INDEX idx_appeals_one_open_per_suspension
    ON appeals(suspension_id)
    WHERE status IN ('open', 'submitted');
