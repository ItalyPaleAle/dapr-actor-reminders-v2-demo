# Dapr Actor Reminders v2 demo

This is a demo of a possible implementation for Dapr Actor Reminders v2 that offers much improved performance.

Although this is implemented as a standalone app for demo purposes, it's intended to be included in each Dapr sidecar, running within the same process. This demo also includes a HTTP server to create and delete reminders; this will not be needed in an actual Dapr sidecar.

# Running the demo

First, clone the repo:

```sh
git clone https://gist.github.com/38014830a5ed1586e8249b8204cea879.git reminders-demo
cd reminders-demo
```

You can then start as many reminder processors as you'd like, each in its terminal window. Just make sure you configure the server to listen on a different port with the `PORT` env var.

For example, to run 2 instances, use 2 terminal windows:

```sh
# First terminal
PORT=3000 go run .

# Second terminal
PORT=3001 go run .
```

You can then create reminders by making requests to `POST /reminder`. Take a look at [`test.sh`](./test.sh) for an example that demonstrates how the solution works.

# Design

This solution requires a relational database. This demo implements SQLite only, but any (most?) relational databases can be used.

However, this solution allows an "unlimited" number of processes (Dapr sidecars) to process reminders, in a conflict-free way. It's ok for processors to scale horizontally, also dynamically. There's a "natural" load balancing thanks to the fact that all processors are competing to fetch reminders from the database.

This solution should allow for really high throughput in executng, scheduling, or re-scheduling (modifying or deleting) reminders. In itself, it's not impacted by the total number of actor types and/or actor IDs, and it can scale horizontally well when there are many reminders to be executed.

## How it works

- Each sidecar maintains in memory a queue (implemented as a priority queue) with the reminders that are scheduled to be executed in the immediate future. This queue is managed by the [Processor](./processor.go) that has one goroutine waiting until the time the reminder is to be executed.
  - For details, see [dapr/dapr#6040](https://github.com/dapr/dapr/pull/6040)
- Periodically every `pollInterval` (in the demo, every 2.5s), the sidecar polls the database to retrieve the next reminder that needs to be executed within the `fetchAhead` interval (in the demo, 5s).
  - At most 1 reminder is retrieved.
    - The query that retrieves the reminder also _atomically_ updates the row storing the current time as `lease_time`. This is used as a "lease token".
    - Rows that have a `lease_time` that is newer than the current time less `leaseDuration` (in the demo, 30s - this must be much bigger than `fetchAhead`) are skipped. This allows making sure that only one sidecar will retrieve a reminder, and if that sidecar is terminated before the reminder is executed, after `leaseDuration` it can be picked up by another sidecar.
    - Right now, the demo code doesn't do any filtering, but it's possible to make this filter only for reminders for actor types that are hosted by the sidecar, and possibly even for the actor IDs that are active.
  - The reminder that is retrieved is added to the in-memory queue to be executed.
- When it's time to execute the reminder:
  1. First, the sidecar starts a transaction in the database which is rolled back automatically if the reminder fails to be executed (which means the reminder's lease will eventually expire and another sidecar will grab it).
  2. The sidecar deletes the reminder from the database (within the transaction).
     - For reminders that are repeating and whose TTL isn't expired, they are not deleted; instead, their `execution_time` is updated to the next iteration.
  3. The reminder is executed.
  4. The transaction is committed if everything went well, which actually deletes the reminder from the database.
- When a new reminder is added, it's saved in the database. If it's scheduled to be executed "immediately", the first sidecar that is polling for reminders will pick it up.
- When a reminder is updated (same actor type, actor ID, and reminder name), it's replaced in the database. This also removes any lease that may exist.

# TODO

- [ ] For databases that support row-level locking (not SQLite), rather than obtaining leases that are stored in the rows, obtain exclusive locks on rows. This allows for quicker detection of expired locks.
- [ ] For databases that support notifications (e.g. Postgres), use those to notify if a new record has been added that needs to be executed within `fetchAhead`.
