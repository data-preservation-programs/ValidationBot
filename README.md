# Validation Bot

This repo contains the validation bot that is used to collect metrics against storage providers to determine a base level of quality to meassure storage proivder operations at.

# Design

# Dispatcher

The dispatcher is responsible for querying the RDS database for all tasks that are ready to be processed and broadcasting them through the pubsub channel to be consumed by the [Auditors](##Auditor) network. It also receives one off tasks from the [tasks API](#Tasks-API) and runs the Auditor module on the recieved task.

# Auditor

The Auditor is responsible for running **validation modules** on the various tasks that it recieves and publishes the **validation results** to Web3Storage. A module's purpose is to test different aspects of a storage provider. These modules include:

* Echo - returns the task input to test its received.

* Trace Route - tests approximate geolocation of miner.

* QueryAsk - query marketplace over libp2p for miner details like Price, PieceSize, etc.

* Index Provider - query [index provider](https://github.com/ipni/storetheindex) for root CID for provider

* Retrieval - creating a timeline of events around stats such as time to first byte (TTFB), bytes downloaded, average speed per sec, time elapsed
  * GraphSync - fetches a random CID from the target miner over [GraphSync]()
  * Bitswap - fetches a random CID from the target miner over [Bitswapo]()
  * Http - fetches a random CID from the target miner over [Http]()

# Trust Manager

The trust manager connects the Auditor and Observer and shares the "trusted peers" so that the Observers and Auditors can coordinate against the Database and Web3Storage to resolve difference between the database CID and the w3s Root CID associated with a miner.

# Observer

The Observer subscribes to Web3Storage and pushes data from validation results back to the RDS database. It does this one CID at a time starting with the last CID seen in the RDS database and compares that to the latest CID recieved by querying the ipns for the miner. The observer will then iterate the difference between last seen CID to the latest Root CID, downloading all entries in between and syncing them to the RDS database.

<iframe width="768" height="432" src="https://miro.com/app/embed/uXjVP2sy1Nk=/?pres=1&frameId=3458764542072189907&embedId=928848589575" frameborder="0" scrolling="no" allow="fullscreen; clipboard-read; clipboard-write" allowfullscreen></iframe>


# Current Project Status

This project is running in production and is in active development.

# Tasks API

The response values will error is unsuccesful and return `nil` when successful. The routes for create, deleting, and listing tasks for a `Dispatcher` are as follows:

```
DELETE /task/:id => error
POST /task Definition => error
GET /tasks => (Definition[], error)
```

The `CREATE` route takes a body with a task `Definition`.

```
struct Definition {
  ID              DefinitionID
  Target          string
  Type            Type
  IntervalSeconds uint32
  Definition      pgtype.JSONB
  Tag             string
}
```

# Validation Bot CLI commands

# Validation Bot Configuration
