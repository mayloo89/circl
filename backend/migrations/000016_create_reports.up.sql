CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reported_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(50) NOT NULL CHECK (reason IN ('harassment', 'spam', 'inappropriate_content', 'fake_profile', 'other')),
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'reviewed', 'dismissed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT no_self_report CHECK (reporter_id != reported_user_id)
);

CREATE INDEX idx_reports_reported_user ON reports(reported_user_id, created_at DESC);
CREATE INDEX idx_reports_status ON reports(status, created_at DESC);
