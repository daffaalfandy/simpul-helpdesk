-- +goose Up

-- "Which system?" on incident reports: an org-wide list admins maintain.
CREATE TABLE systems (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  name        text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
  description text CHECK (length(description) BETWEEN 1 AND 300),
  sort_order  integer NOT NULL DEFAULT 0 CHECK (sort_order BETWEEN 0 AND 10000),
  active      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX systems_name_key ON systems (lower(name));
CREATE TRIGGER systems_updated_at BEFORE UPDATE ON systems
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- "What do you need?" on service requests. delivery_minutes are working minutes; they become the
-- delivery target when the priority's SLA target has deliver_from_catalog.
CREATE TABLE catalog_items (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  name             text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
  description      text CHECK (length(description) BETWEEN 1 AND 300),
  delivery_minutes integer NOT NULL CHECK (delivery_minutes BETWEEN 1 AND 525600),
  sort_order       integer NOT NULL DEFAULT 0 CHECK (sort_order BETWEEN 0 AND 10000),
  active           boolean NOT NULL DEFAULT true,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX catalog_items_name_key ON catalog_items (lower(name));
CREATE TRIGGER catalog_items_updated_at BEFORE UPDATE ON catalog_items
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Ticket numbers. Gaps (rolled-back inserts) are fine.
CREATE SEQUENCE ticket_number_seq AS bigint START 1;

CREATE TABLE tickets (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  number            bigint NOT NULL CHECK (number > 0) CONSTRAINT tickets_number_key UNIQUE,
  -- Prefix + number, fixed at creation so a later prefix change never rewrites old keys.
  key               text NOT NULL CONSTRAINT tickets_key_key UNIQUE,
  type              text NOT NULL CHECK (type IN ('incident', 'request')),
  status            text NOT NULL CHECK (status IN ('new', 'assigned', 'in_progress', 'waiting_on_client',
                                                    'in_uat', 'resolved', 'closed')),
  priority          text NOT NULL CHECK (priority IN ('P1', 'P2', 'P3', 'P4')),
  channel           text NOT NULL CHECK (channel IN ('portal', 'phone', 'email', 'whatsapp', 'in_person')),
  client_id         uuid NOT NULL CONSTRAINT tickets_client_id_fkey REFERENCES clients (id) ON DELETE RESTRICT,
  requester_id      uuid NOT NULL CONSTRAINT tickets_requester_id_fkey REFERENCES users (id) ON DELETE RESTRICT,
  assignee_id       uuid CONSTRAINT tickets_assignee_id_fkey REFERENCES users (id) ON DELETE RESTRICT,
  created_by        uuid NOT NULL CONSTRAINT tickets_created_by_fkey REFERENCES users (id) ON DELETE RESTRICT,
  system_id         uuid CONSTRAINT tickets_system_id_fkey REFERENCES systems (id) ON DELETE RESTRICT,
  catalog_item_id   uuid CONSTRAINT tickets_catalog_item_id_fkey REFERENCES catalog_items (id) ON DELETE RESTRICT,
  subject           text NOT NULL CHECK (length(subject) BETWEEN 1 AND 200),
  description       text NOT NULL CHECK (length(description) BETWEEN 1 AND 20000),
  -- The client's SLA plan targets as they were at creation (internal/ticket.SLASnapshot).
  sla_snapshot      jsonb NOT NULL CHECK (jsonb_typeof(sla_snapshot) = 'object'),
  first_response_at timestamptz,
  resolved_at       timestamptz,
  closed_at         timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT tickets_key_format CHECK (key ~ '^[A-Z][A-Z0-9]{0,9}-[1-9][0-9]*$'),
  CONSTRAINT tickets_key_matches_number CHECK (right(key, length(number::text) + 1) = '-' || number::text),
  -- Incidents name a system; service requests name a catalogue item.
  CONSTRAINT tickets_type_detail CHECK (
    (type = 'incident' OR system_id IS NULL) AND (type = 'request' OR catalog_item_id IS NULL)),
  CONSTRAINT tickets_new_unassigned CHECK (status <> 'new' OR assignee_id IS NULL),
  CONSTRAINT tickets_assigned_has_assignee CHECK (status <> 'assigned' OR assignee_id IS NOT NULL),
  CONSTRAINT tickets_resolved_at CHECK ((resolved_at IS NOT NULL) = (status IN ('resolved', 'closed'))),
  CONSTRAINT tickets_closed_at CHECK ((closed_at IS NOT NULL) = (status = 'closed'))
);
ALTER SEQUENCE ticket_number_seq OWNED BY tickets.number;
CREATE INDEX tickets_client_id_idx ON tickets (client_id);
CREATE INDEX tickets_requester_id_idx ON tickets (requester_id);
CREATE INDEX tickets_status_idx ON tickets (status);
CREATE INDEX tickets_assignee_id_idx ON tickets (assignee_id) WHERE assignee_id IS NOT NULL;
CREATE INDEX tickets_system_id_idx ON tickets (system_id) WHERE system_id IS NOT NULL;
CREATE INDEX tickets_catalog_item_id_idx ON tickets (catalog_item_id) WHERE catalog_item_id IS NOT NULL;
CREATE TRIGGER tickets_updated_at BEFORE UPDATE ON tickets
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Conversation entries. Internal notes are never shown to requesters. Messages are not edited.
CREATE TABLE ticket_messages (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  ticket_id  uuid NOT NULL CONSTRAINT ticket_messages_ticket_id_fkey REFERENCES tickets (id) ON DELETE RESTRICT,
  author_id  uuid NOT NULL CONSTRAINT ticket_messages_author_id_fkey REFERENCES users (id) ON DELETE RESTRICT,
  visibility text NOT NULL CHECK (visibility IN ('public', 'internal')),
  body       text NOT NULL CHECK (length(body) BETWEEN 1 AND 20000),
  created_at timestamptz NOT NULL DEFAULT now(),
  -- Target of attachments_message_fkey: an attachment's message belongs to its ticket.
  CONSTRAINT ticket_messages_id_ticket_key UNIQUE (id, ticket_id)
);
CREATE INDEX ticket_messages_ticket_id_idx ON ticket_messages (ticket_id, created_at);

-- Uploads stay pending (ticket_id NULL) until linked; linked without a message means the description's
-- file. storage_key names the file in internal/storage and never derives from the filename.
CREATE TABLE attachments (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  storage_key  text NOT NULL CONSTRAINT attachments_storage_key_key UNIQUE,
  filename     text NOT NULL CHECK (length(filename) BETWEEN 1 AND 255),
  content_type text NOT NULL CHECK (length(content_type) BETWEEN 1 AND 100),
  -- 10485760 = 10 MiB.
  size_bytes   bigint NOT NULL CHECK (size_bytes BETWEEN 1 AND 10485760),
  -- 32 bytes = SHA-256.
  sha256       bytea NOT NULL CHECK (octet_length(sha256) = 32),
  uploaded_by  uuid NOT NULL CONSTRAINT attachments_uploaded_by_fkey REFERENCES users (id) ON DELETE RESTRICT,
  ticket_id    uuid CONSTRAINT attachments_ticket_id_fkey REFERENCES tickets (id) ON DELETE RESTRICT,
  message_id   uuid,
  created_at   timestamptz NOT NULL DEFAULT now(),
  linked_at    timestamptz,
  CONSTRAINT attachments_message_fkey FOREIGN KEY (message_id, ticket_id)
    REFERENCES ticket_messages (id, ticket_id) ON DELETE RESTRICT,
  CONSTRAINT attachments_message_needs_ticket CHECK (message_id IS NULL OR ticket_id IS NOT NULL),
  CONSTRAINT attachments_linked_at CHECK ((ticket_id IS NULL) = (linked_at IS NULL))
);
CREATE INDEX attachments_ticket_id_idx ON attachments (ticket_id) WHERE ticket_id IS NOT NULL;
CREATE INDEX attachments_message_id_idx ON attachments (message_id) WHERE message_id IS NOT NULL;
CREATE INDEX attachments_pending_idx ON attachments (created_at) WHERE ticket_id IS NULL;
CREATE INDEX attachments_pending_uploader_idx ON attachments (uploaded_by) WHERE ticket_id IS NULL;

-- +goose Down
DROP TABLE attachments;
DROP TABLE ticket_messages;
DROP TABLE tickets;
DROP TABLE catalog_items;
DROP TABLE systems;
