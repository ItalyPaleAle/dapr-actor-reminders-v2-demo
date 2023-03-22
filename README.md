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

You can then create reminders by making requests to `POST /reminder`. Take a look at [`test.sh`](./test.sh) for an example that demonstrates how the solution works (assumes apps listening on ports 3000 and 3001).

# Design

This solution requires a relational database. This demo implements SQLite only, but any (most?) relational databases can be used.

However, this solution allows an "unlimited" number of processes (Dapr sidecars) to process reminders, in a conflict-free way. It's ok for processors to scale horizontally, also dynamically. There's a "natural" load balancing thanks to the fact that all processors are competing to fetch reminders from the database.

This solution should allow for really high throughput in executing, scheduling, or re-scheduling (modifying or deleting) reminders. In itself, it's not impacted by the total number of actor types and/or actor IDs, and it can scale horizontally well when there are many reminders to be executed. The goal is that the limiting factor for performance and scalability should only be in the database, and not in Dapr itself.

Unlike other proposals for "v2" of Actor Reminders in Dapr, this one doesn't involve creating a separate control plane service for storing and executing Actor Reminders.

## How it works

- Each sidecar maintains in memory a queue (implemented as a priority queue) with the reminders that are scheduled to be executed in the immediate future. This queue is managed by the [Processor](./pkg/reminders/processor.go) that has one goroutine waiting until the time the reminder is to be executed.
  - For details, see [dapr/dapr#6040](https://github.com/dapr/dapr/pull/6040)
- Periodically every `pollInterval` (in the demo, every 2.5s), the sidecar polls the database to retrieve the next reminders that needs to be executed within the `fetchAhead` interval (in the demo, 5s).
  - At most `batchSize` (in the demo, 2) reminders are retrieved, and they are all scheduled to be executed within `fetchAhead`.
    - The query that retrieves the reminders also _atomically_ updates the rows storing the current time as `lease_time`. This is used as a "lease token".
    - Rows that have a `lease_time` that is newer than the current time less `leaseDuration` (in the demo, 30s - this must be much bigger than `fetchAhead`) are skipped. This allows making sure that only one sidecar will retrieve a reminder, and if that sidecar is terminated before the reminder is executed, after `leaseDuration` it can be picked up by another sidecar.
    - Right now, the demo code doesn't do any filtering, but it's possible to make this filter only for reminders for actor types that are hosted by the sidecar, and possibly even for the actor IDs that are active.
  - The reminders that are retrieved are added to the in-memory queue to be executed at the time they're scheduled for.
- When it's time to execute the reminder:
  1. First, the sidecar starts a transaction in the database which is rolled back automatically if the reminder fails to be executed (which means the reminder's lease will eventually expire and another sidecar will grab it).
  2. The sidecar deletes the reminder from the database (within the transaction).
     - For reminders that are repeating and whose TTL isn't expired, they are not deleted; instead, their `execution_time` is updated to the next iteration.
  3. The reminder is executed.
  4. The transaction is committed if everything went well, which actually deletes the reminder from the database.
- When a new reminder is added, it's saved in the database. If it's scheduled to be executed "immediately", the first sidecar that is polling for reminders will pick it up.
  - If the reminder's scheduled time is within `fetchAhead` from now (in the demo, 5s), then it's stored in the database in a way that is already owned by the current sidecar (e.g. with `lease_time` already set). It's then directly enqueued in the queue managed by the current sidecar.
  - This behavior can potentially lead to a less uniform distribution of reminders, so users should have a way to disable it.
  - _Note: this is **not** implemented in this demo for the reason above_
- When a reminder is updated (same actor type, actor ID, and reminder name), it's replaced in the database. This also removes any lease that may exist.

# Notes for implementing in Dapr

When implementing this in Dapr:

- As mentioned, this proposal requires Dapr to have access to a relational database.
  - We will add relevant methods to state store components that have the required capabilities (pretty much all the relational databases, and only those). These methods are going to be very specific for our use case.
    - For example, components will implement a `WatchReminders` method that sends on a Go channel all reminders as they come in. The method is generic because each component will have a different implementation depending on the capabilities of the database. For example, whilst SQLite will do a periodic polling, for Postgres we could rely on PGNotify to be notified when new reminders are added that are within the `fetchAhead` interval instead, avoiding polling.
    - Another example is how we acquire leases. For databases that support row-level locking (not SQLite), rather than obtaining leases that are stored in the rows, obtain exclusive locks on rows. This allows for quicker detection of expired locks too.
    - Another example of a method that will be component-specific is `ExecuteReminders`. The component will manage the long-running transaction and the caller (daprd) will pass some callbacks that are invoked when the row is deleted (or updated for repeating reminders) and that method returns to confirm the execution. For databases like SQLite in which long-running transactions aren't an option (because transactions block the entire database), we will instead have a background loop that renews the lease while the reminder is being executed.
- With this proposal, reminders are still executed by each sidecar, just like in the current "v1" (and unlike other "v2" proposals that involved creating a separate control plane service).  
  Because of this, we may be able to continue offering users the option of using the "v1" implementation (with caveats around its performance and scalability) if they have the requirement of continuing to use state stores that are not relational databases (assuming we're comfortable with the cost of continuing to support the then-legacy solution).
