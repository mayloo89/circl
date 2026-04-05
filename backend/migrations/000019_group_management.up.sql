-- Track who created a group so admin-only operations can be authorised.
-- DM rooms leave creator_id NULL.
ALTER TABLE rooms ADD COLUMN creator_id UUID REFERENCES users(id);
