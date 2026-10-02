-- Exact revision bindings invalidate approval when either environment changes.
CREATE TABLE vault_promotion_bindings (
 promotion_id UUID PRIMARY KEY REFERENCES promotion_requests(id) ON DELETE RESTRICT,
 project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 request_digest TEXT NOT NULL,
 source_digest TEXT NOT NULL,
 target_digest TEXT NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE vault_promotion_effects (
 promotion_id UUID NOT NULL REFERENCES vault_promotion_bindings(promotion_id) ON DELETE RESTRICT,
 project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
 secret_id UUID NOT NULL,
 applied_revision BIGINT NOT NULL CHECK(applied_revision>0),
 PRIMARY KEY(promotion_id,secret_id),
 FOREIGN KEY(secret_id,project_id) REFERENCES vault_entries(secret_id,project_id) ON DELETE RESTRICT
);

CREATE TRIGGER vault_promotion_bindings_immutable BEFORE UPDATE OR DELETE ON vault_promotion_bindings FOR EACH ROW EXECUTE FUNCTION keepsave_vault_immutable();
CREATE TRIGGER vault_promotion_effects_immutable BEFORE UPDATE OR DELETE ON vault_promotion_effects FOR EACH ROW EXECUTE FUNCTION keepsave_vault_immutable();

CREATE FUNCTION keepsave_promotion_parameters_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM vault_promotion_bindings WHERE promotion_id=OLD.id) AND
 (NEW.project_id IS DISTINCT FROM OLD.project_id OR NEW.source_environment IS DISTINCT FROM OLD.source_environment OR NEW.target_environment IS DISTINCT FROM OLD.target_environment OR NEW.requested_by IS DISTINCT FROM OLD.requested_by OR NEW.keys_filter IS DISTINCT FROM OLD.keys_filter OR NEW.override_policy IS DISTINCT FROM OLD.override_policy OR NEW.notes IS DISTINCT FROM OLD.notes OR NEW.created_at IS DISTINCT FROM OLD.created_at) THEN
  RAISE EXCEPTION 'bound promotion parameters are immutable';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER promotion_parameters_immutable BEFORE UPDATE ON promotion_requests FOR EACH ROW EXECUTE FUNCTION keepsave_promotion_parameters_immutable();
