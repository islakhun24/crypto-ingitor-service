# Database Migrations

Phase 2 creates derivative aggregation tables only. The existing `symbols` table is not recreated or altered.

Apply these files in filename order:

1. `000001_exchange_api_endpoints.sql`
2. `000002_scheduler_tables.sql`
3. `000003_core_derivative_tables.sql`
4. `000004_advanced_derivative_tables.sql`
5. `000005_monitoring_quality_tables.sql`
6. `000006_retention_rollup_tables.sql`
7. `000007_seed_exchange_api_endpoints.sql`
8. `000008_seed_derivative_collection_policies.sql`
9. `000009_aggregate_snapshot_columns.sql`
10. `000010_phase7_advanced_columns.sql`
11. `000011_phase8_analytics_layer.sql`
12. `000012_phase9_retention_policy_seed.sql`
13. `000013_phase10_production_hardening.sql`
14. `000014_phase11_api_indexes.sql`

Every table and index uses idempotent DDL with `IF NOT EXISTS`. High-growth snapshot, candle, job, payload, gap, and run tables include unique indexes for idempotent writes.

Endpoint seed rows use `ON CONFLICT (exchange, market_type, data_type, name) DO UPDATE` so they can be applied repeatedly as endpoint metadata evolves.

Collection policy seeds use `ON CONFLICT DO NOTHING` and cover the default `all`, `top100`, and `watchlist` tiers.

The aggregate compatibility migration adds Phase 6 aggregate columns while preserving the original table.

Phase 7 extends advanced derivative tables and aggregate snapshots for flow, CVD, liquidation, basis, orderbook imbalance, and divergence.

Phase 8 adds analytics-oriented JSONB metrics, quality metadata, anomaly flags, market structure fields, volatility fields, and query indexes. It intentionally does not create signal scoring tables or recommendation outputs.

Phase 9 seeds explicit retention policies for high-growth derivative, orderbook, liquidation, and debug tables. Policies default to `dry_run=true`; cleanup uses policy-scoped chunked deletes, kline rollups before low-timeframe deletion, audit run tables, and partition-drop support when partitioned tables are present.

Phase 10 adds production hardening metadata for backfill/recovery, detailed dead-letter rows, data-quality quarantine provenance, and indexed gap/recovery lookups.

Phase 11 adds read-path indexes for the frontend derivative terminal REST API. The API reads from latest aggregate and indexed series tables through stable DTOs instead of exposing raw table schemas.

Phase 12 is runtime-only: realtime websocket/latest events are kept in Redis or memory under `deriv:*` keys, with periodic snapshot buffering for any DB persistence. It does not add raw websocket event tables.

Phase 13 does not add migrations. It packages the existing schema for Docker Compose deployment and keeps migrations applied through `make migrate`.
