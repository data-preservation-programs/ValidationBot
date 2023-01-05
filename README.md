# Validation Bot

This repo contains the validation bot that is used to collect metrics to determine a base meassurement for quality of miners on the Filecoin network.

# Current Project Status

This project is running in production and is in active development.

# Validation Bot CLI commands

TODO

# Configuration

TODO

# Design

# Architecture

![validation-bot-architechture](/assets/validation-bot-architecture.jpg)
[miro board](https://miro.com/app/board/uXjVP2sy1Nk=/?moveToViewport=-1582,-828,2605,2080&embedId=755140909102)

## Dispatcher

The dispatcher is responsible for querying the RDS database for all tasks that are ready to be processed and broadcasting them through the pubsub channel (via libp2p) to be consumed by the [Auditors](##Auditor) network. It also receives one off tasks from the [tasks API](#Tasks-API) and runs the Auditor module on the recieved task.

## Auditor

The Auditor is responsible for running **validation modules** on the various tasks that it recieves, publishing the **validation results** to Web3Storage. A module's purpose is to test different aspects of a storage provider. These modules include:

* Echo - returns the task input to test its received.
* Trace Route - tests approximate geolocation of miner.
* QueryAsk - queries **marketplace** (via libp2p) for miner details like Price, PieceSize, etc.
* Index Provider - query [index provider](https://github.com/ipni/storetheindex) for root CID of miner.
* Retrieval - creating a timeline of events around stats such as time to first byte (TTFB), bytes downloaded, average speed per sec, time elapsed, etc.
  * GraphSync - fetches a random CID from the target miner over [GraphSync](https://github.com/ipld/specs/blob/master/block-layer/graphsync/graphsync.md)
  * Bitswap - fetches a random CID from the target miner over [Bitswap](https://github.com/ipfs/specs/blob/main/BITSWAP.md)
  * Http - fetches a random CID from the target miner over [Http](https://docs.ipfs.tech/reference/http/gateway/)

## Trust Manager

The trust manager connects the Auditor and Observer, sharing "trusted peers" so that the Observers and Auditors can coordinate against the Database and Web3Storage to resolve difference between a miners database CID and the miner's Web3Storage Root CID.

## Observer

The Observer subscribes to Web3Storage and pushes data from the **validation results** back to the **RDS database**. It does this one CID at a time starting with the **last CID seen in the RDS database**, comparing that to the **latest CID recieved by querying the ipns for the miner**. The observer will then iterate the difference between last seen CID to the latest Root CID, downloading all entries in between and syncing them to the RDS database.

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
