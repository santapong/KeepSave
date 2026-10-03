-- Capture known scope only when a new run is admitted. Existing NULL snapshots
-- remain historical unknowns; the application refuses provider work on them.
ALTER TABLE tool_runs ADD COLUMN scope JSONB CHECK(scope IS NULL OR jsonb_typeof(scope)='object');

CREATE FUNCTION keepsave_tool_run_scope_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.scope IS DISTINCT FROM OLD.scope OR NEW.reference IS DISTINCT FROM OLD.reference THEN
  RAISE EXCEPTION 'immutable admitted run scope';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER tool_run_scope_immutable BEFORE UPDATE ON tool_runs FOR EACH ROW EXECUTE FUNCTION keepsave_tool_run_scope_immutable();
