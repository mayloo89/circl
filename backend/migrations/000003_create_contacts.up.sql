CREATE TABLE contacts (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                             CHECK (status IN ('pending', 'accepted', 'blocked')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- A user can only have one relationship entry with another user.
    CONSTRAINT uq_contacts_pair UNIQUE (requester_id, addressee_id),
    -- A user cannot add themselves as a contact.
    CONSTRAINT chk_contacts_no_self CHECK (requester_id <> addressee_id)
);

CREATE INDEX idx_contacts_requester ON contacts(requester_id);
CREATE INDEX idx_contacts_addressee ON contacts(addressee_id);
