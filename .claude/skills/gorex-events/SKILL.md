---
name: gorex-events
description: Query GoRex's wide events, its spec and its test results with DuckDB, to see what the system actually does - debugging a misbehaviour, checking that a feature works in real use, finding slow or failing requests, what agents do with rex, or which spec items lack tests. Use before guessing at a cause, and when asked how GoRex is used or behaves.
---

# GoRex's wide events

The server and the `rex` CLI write one flat JSON line per unit of work to
`$GOREX_DIR/events/YYYY-MM-DD.jsonl` (`~/.config/GoRex/events/` by
default, kept 30 days). Many columns, few of them set on each line: ask
questions nobody planned by grouping and filtering on any of them.

```
go run ./tools/spec query "SQL"     # from the repo; tables: events, items, links, tests, problems, test_results
go run ./tools/spec query           # an interactive duckdb
```

Without the repo: `duckdb -c "select … from read_json_auto('~/.config/GoRex/events/*.jsonl', union_by_name=true, sample_size=-1)"`.

## Columns

Every line: `ts` (timestamp, UTC), `kind`, `version`, `protocol`, `pid`,
`os`, `arch`; `dropped_before` when the writer had to drop lines.

**kind = 'request'**, a request the server answered (the GUI's successful
polls are left out):
`op`, `ok`, `code` (not_found, ambiguous, usage, no_client), `error`,
`duration_ms`, `response_bytes`; who: `client_kind` (cli, app, attach…),
`client_id`, `client_name`, `client_pid`; the server then: `revision`,
`n_sessions`, `n_windows`, `n_blocks`, `n_clients`, `uptime_s`; the
arguments: `arg_session`, `arg_window`, `arg_block`, `arg_target`,
`ctx_session`, `ctx_block` (where the CLI ran), `arg_split`,
`arg_direction`, `arg_label`, `arg_ratio`, `arg_index`, `arg_zoom`,
`arg_format`, `arg_trim`, `arg_all`, `arg_model`, `client_target`,
`action`, `data_len` and `n_keys` (the sizes of what was typed, never the
text), `create_program`, `create_shell`, `create_keep_open`,
`create_has_command`, `create_label`; `status_state`, `status_kind`,
`status_app` for status reports.

**kind = 'event'**, an event the server emitted: `type` (session_created,
window_created, block_created, block_event, session_view_changed,
client_joined…), `event_type` (the outer type), `block_event` (for
type = 'block_event': bell, child_exited, title_changed, pwd_changed,
process_changed, program_status_changed…), `session_id`, `block_id`, the
payload's scalar fields as `p_*` (`p_exit_code`, `p_window_id`,
`p_label`, `p_reason`…), a program status record as `status_state`,
`status_kind`, `status_app`, `status_id`, `status_progress`,
`status_msg_len`, and `payload`, the whole payload as JSON text (status
messages and titles left out).

**kind = 'cli'**, a `rex` command: `command` (the words before the first
flag: "split", "server stop", "skill hook claude Stop"), `flags` (their
names only), `exit_code`, `ok`, `error`, `duration_ms`, `n_args`,
`in_rex`, `rex_session`, `rex_block`, `client`, `agent` (claude-code,
codex). Its `pid` is the `client_pid` of the requests it made.

**kind = 'server'**: `event` = start (`exe`, `sessions`, `blocks`) or stop
(`forget`, `blocks`, `uptime_s`).

Columns appear as lines carry them: `describe events` shows today's set.
A column missing from every line is missing from the table: write queries
for one that may not exist yet with `try`, or check `describe` first.

## Queries to start from

```sql
-- what failed, lately
select ts, op, code, error, client_kind from events
where kind='request' and not ok order by ts desc limit 20;

-- a command and the requests it made
select c.ts, c.command, c.exit_code, string_agg(r.op, ' ' order by r.ts) ops
from events c left join events r on r.kind='request' and r.client_pid=c.pid
where c.kind='cli' group by all order by c.ts desc limit 20;

-- slow requests, by op
select op, count(*) n, round(median(duration_ms),2) p50,
       round(quantile_cont(duration_ms,0.99),2) p99, max(duration_ms) max
from events where kind='request' group by op order by p99 desc;

-- what agents do with rex
select agent, command, count(*) n, avg(exit_code<>0)::decimal(4,2) failing
from events where kind='cli' and agent is not null group by all order by n desc;

-- program status as Claude Code reported it, per block
select block_id, status_app, status_state, status_kind, count(*) n from events
where block_event='program_status_changed' group by all order by n desc;

-- blocks that exited badly
select ts, session_id, block_id, p_exit_code from events
where block_event='child_exited' and p_exit_code<>0 order by ts desc;

-- server restarts and their uptimes
select ts, event, uptime_s, version from events where kind='server' order by ts;
```

Spec and tests:

```sql
-- accepted functional items no test verifies
select id, title from items i where layer='functional' and status='accepted'
and verify='test' and id not in (select id from tests);

-- tests that failed in the last run, with the items they verify
select r.Package, r.Test, t.id from test_results r left join tests t on t.test=r.Test
where r.Action='fail' and r.Test is not null;

-- one business need, down to its tests
select b.id, f.id, f.title, t.test from items b
join items f on list_contains(f.traces, b.id)
left join tests t on t.id=f.id where b.id='B-1';
```

## Adding fields

When a question has no column to answer it, add one: in
`internal/rex/wide_server.go` (requests and events) or `wideCommand` in
`cli.go` (commands). Flat keys, set only when meaningful (`setNonZero`),
never text a user typed or a terminal printed. Add it to this list.
