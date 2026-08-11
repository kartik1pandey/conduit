-- The postgres container only auto-creates the database named by POSTGRES_DB
-- (conduit_core). Each service gets its own logical database on the same
-- instance for local dev, so create the rest here. This script only runs
-- once, the first time the postgres data volume is initialized.
CREATE DATABASE conduit_ledger;
CREATE DATABASE conduit_risk;
CREATE DATABASE conduit_billing;
CREATE DATABASE conduit_webhooks;
CREATE DATABASE conduit_dashboard;
