# apps/worker

Standalone Go worker process. Claims runnable workflow tasks from PostgreSQL
with `FOR UPDATE SKIP LOCKED`, executes deterministic built-in actions, and
commits state transitions, attempts and ordered events transactionally.
