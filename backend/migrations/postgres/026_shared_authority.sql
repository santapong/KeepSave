-- Authority epochs survive membership removal. Rejoining never revives grants.
CREATE TABLE member_authority_state (
 organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
 user_id UUID NOT NULL REFERENCES users(id),
 active BOOLEAN NOT NULL,
 epoch BIGINT NOT NULL CHECK(epoch > 0),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,user_id)
);
INSERT INTO member_authority_state(organization_id,user_id,active,epoch)
 SELECT organization_id,user_id,true,1 FROM organization_members;

-- Every legacy writer participates in invalidation, including upserts. A role
-- change changes authority; a no-op update does not invalidate a current grant.
CREATE FUNCTION keepsave_member_epoch() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  UPDATE member_authority_state SET active=false,epoch=epoch+1,updated_at=now()
   WHERE organization_id=OLD.organization_id AND user_id=OLD.user_id;
  RETURN OLD;
 END IF;
 IF TG_OP='UPDATE' AND NEW.role IS NOT DISTINCT FROM OLD.role THEN RETURN NEW; END IF;
 INSERT INTO member_authority_state(organization_id,user_id,active,epoch)
 VALUES(NEW.organization_id,NEW.user_id,true,1)
 ON CONFLICT(organization_id,user_id) DO UPDATE SET active=true,
 epoch=member_authority_state.epoch+1,updated_at=now();
 RETURN NEW;
END $$;
CREATE TRIGGER member_authority_epoch AFTER INSERT OR UPDATE OF role OR DELETE
 ON organization_members FOR EACH ROW EXECUTE FUNCTION keepsave_member_epoch();
