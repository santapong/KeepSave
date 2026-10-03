-- History is immutable, including actor identifiers after account removal.
ALTER TABLE vault_revisions DROP CONSTRAINT vault_revisions_actor_id_fkey;
CREATE FUNCTION keepsave_vault_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'vault history is immutable';
END;
$$;
CREATE TRIGGER vault_revisions_immutable BEFORE UPDATE OR DELETE ON vault_revisions
 FOR EACH ROW EXECUTE FUNCTION keepsave_vault_immutable();

-- Tenant identity is carried by composite relationships, not trusted inputs.
ALTER TABLE environments ADD CONSTRAINT environments_id_project_unique UNIQUE(id,project_id);
ALTER TABLE vault_entries ADD CONSTRAINT vault_entries_id_project_unique UNIQUE(secret_id,project_id);
ALTER TABLE vault_entries ADD CONSTRAINT vault_entries_environment_project_fk
 FOREIGN KEY(environment_id,project_id) REFERENCES environments(id,project_id);
ALTER TABLE vault_revisions ADD CONSTRAINT vault_revisions_entry_project_fk
 FOREIGN KEY(secret_id,project_id) REFERENCES vault_entries(secret_id,project_id);

-- An enrolled project's compatibility table may only change in the same
-- transaction as the current journal revision. Deferred checks allow both
-- journal-first and compatibility-first adapters, but prevent old SQL writers.
CREATE FUNCTION keepsave_vault_check_live() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE target_id uuid; target_project uuid;
BEGIN
 IF TG_OP='DELETE' THEN target_id:=OLD.id; target_project:=OLD.project_id;
 ELSE target_id:=NEW.id; target_project:=NEW.project_id; END IF;
 IF EXISTS(SELECT 1 FROM vault_projects WHERE project_id=target_project) THEN
  IF EXISTS(SELECT 1 FROM secrets WHERE id=target_id) THEN
   IF NOT EXISTS(
    SELECT 1 FROM secrets s JOIN vault_entries e ON e.secret_id=s.id AND e.project_id=s.project_id
     JOIN vault_revisions r ON r.secret_id=e.secret_id AND r.project_id=e.project_id AND r.revision=e.revision
    WHERE s.id=target_id AND NOT e.deleted AND e.environment_id=s.environment_id AND e.secret_key=s.key
     AND r.key_id=e.key_id AND r.encrypted_value=s.encrypted_value AND r.nonce=s.value_nonce AND r.operation<>'delete'
   ) THEN RAISE EXCEPTION 'enrolled vault write requires matching journal'; END IF;
  ELSIF NOT EXISTS(SELECT 1 FROM vault_entries WHERE secret_id=target_id AND project_id=target_project AND deleted) THEN
   RAISE EXCEPTION 'enrolled vault deletion requires a tombstone';
  END IF;
 END IF;
 RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER vault_live_journal_guard AFTER INSERT OR UPDATE OR DELETE ON secrets
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION keepsave_vault_check_live();

-- Isolated recovery keeps ciphertext snapshots recoverable without restoring
-- provider grants, promotion authority, users or other live workflow records.
CREATE TABLE vault_recovery_snapshots (
 id UUID PRIMARY KEY,
 project_id UUID NOT NULL,
 key_id UUID NOT NULL,
 source_promotion_id UUID NOT NULL,
 environment_id UUID NOT NULL,
 secret_key VARCHAR(255) NOT NULL,
 encrypted_value BYTEA NOT NULL,
 nonce BYTEA NOT NULL,
 FOREIGN KEY(key_id,project_id) REFERENCES vault_keys(id,project_id),
 FOREIGN KEY(environment_id,project_id) REFERENCES environments(id,project_id)
);
CREATE TRIGGER vault_recovery_snapshots_immutable BEFORE UPDATE OR DELETE ON vault_recovery_snapshots
 FOR EACH ROW EXECUTE FUNCTION keepsave_vault_immutable();

-- Wrapped key material and snapshot references are retained, never replaced.
-- New DEKs are new rows; legacy master-key rotation is unavailable in core.
CREATE TRIGGER vault_keys_immutable BEFORE UPDATE OR DELETE ON vault_keys
 FOR EACH ROW EXECUTE FUNCTION keepsave_vault_immutable();
CREATE TRIGGER vault_snapshot_keys_immutable BEFORE UPDATE OR DELETE ON vault_snapshot_keys
 FOR EACH ROW EXECUTE FUNCTION keepsave_vault_immutable();
CREATE FUNCTION keepsave_vault_snapshot_binding() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM secret_snapshots ss JOIN promotion_requests pr ON pr.id=ss.promotion_id
  WHERE ss.id=NEW.snapshot_id AND pr.project_id=NEW.project_id AND ss.prior_existed) THEN
  RAISE EXCEPTION 'invalid snapshot project binding';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER vault_snapshot_keys_owner BEFORE INSERT ON vault_snapshot_keys
 FOR EACH ROW EXECUTE FUNCTION keepsave_vault_snapshot_binding();
CREATE FUNCTION keepsave_vault_protect_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM promotion_requests pr JOIN vault_projects vp ON vp.project_id=pr.project_id WHERE pr.id=OLD.promotion_id) THEN
  RAISE EXCEPTION 'enrolled vault snapshot is immutable';
 END IF;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$;
CREATE TRIGGER vault_snapshots_immutable BEFORE UPDATE OR DELETE ON secret_snapshots
 FOR EACH ROW EXECUTE FUNCTION keepsave_vault_protect_snapshot();

-- Journal pointer changes must also agree with compatibility values at commit;
-- guarding the live table alone would miss a writer touching only the journal.
CREATE FUNCTION keepsave_vault_check_entry() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE current_entry vault_entries%ROWTYPE;
BEGIN
 SELECT * INTO current_entry FROM vault_entries WHERE secret_id=NEW.secret_id AND project_id=NEW.project_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'vault entry missing'; END IF;
 IF current_entry.deleted THEN
  IF EXISTS(SELECT 1 FROM secrets WHERE id=NEW.secret_id) OR NOT EXISTS(
    SELECT 1 FROM vault_revisions WHERE secret_id=NEW.secret_id AND project_id=NEW.project_id
     AND revision=current_entry.revision AND key_id=current_entry.key_id AND operation='delete') THEN
   RAISE EXCEPTION 'vault tombstone requires deleted current value';
  END IF;
 ELSIF NOT EXISTS(
  SELECT 1 FROM vault_entries e JOIN secrets s ON s.id=e.secret_id AND s.project_id=e.project_id
   JOIN vault_revisions r ON r.secret_id=e.secret_id AND r.project_id=e.project_id AND r.revision=e.revision
  WHERE e.secret_id=NEW.secret_id AND NOT e.deleted AND e.environment_id=s.environment_id AND e.secret_key=s.key
   AND e.key_id=r.key_id AND r.encrypted_value=s.encrypted_value AND r.nonce=s.value_nonce AND r.operation<>'delete'
 ) THEN RAISE EXCEPTION 'vault current pointer requires matching value'; END IF;
 RETURN NULL;
END;
$$;
CREATE CONSTRAINT TRIGGER vault_entry_journal_guard AFTER INSERT OR UPDATE ON vault_entries
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION keepsave_vault_check_entry();

CREATE FUNCTION keepsave_vault_entry_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.secret_id<>OLD.secret_id OR NEW.project_id<>OLD.project_id OR NEW.environment_id<>OLD.environment_id OR NEW.secret_key<>OLD.secret_key THEN
  RAISE EXCEPTION 'vault entry identity is immutable';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER vault_entry_identity_immutable BEFORE UPDATE ON vault_entries
 FOR EACH ROW EXECUTE FUNCTION keepsave_vault_entry_identity();
CREATE FUNCTION keepsave_vault_live_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM vault_projects WHERE project_id=OLD.project_id) AND
   (NEW.id<>OLD.id OR NEW.project_id<>OLD.project_id OR NEW.environment_id<>OLD.environment_id OR NEW.key<>OLD.key) THEN
  RAISE EXCEPTION 'enrolled vault value identity is immutable';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER vault_live_identity_immutable BEFORE UPDATE ON secrets
 FOR EACH ROW EXECUTE FUNCTION keepsave_vault_live_identity();
