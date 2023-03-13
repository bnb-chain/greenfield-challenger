# Greenfield Challenger
Greenfield ensures data integrity by routinely issuing storage providers challenge events to prove that the stored data is not tampered. This service allows end users to monitor the blockchain for challenge events and conduct verification upon downloading the hash pieces belonging to the stored object. 

## How it works
This off-chain application comprises of 4 working parts: Monitor, Verifier, Vote Processor and Tx Submitter. 
1. The Monitor polls the blockchain for new challenge events and adds them to the local db for further processing.


2. The Verifier would then retrieve the event from the db before querying the Storage Provider for the piece hashes and the Blockchain for the original hash. A root hash would be computed using the piece hashes received from the Storage Provider. Both the root hash and original hash would then be compared to check if they are equal before updating the db with the challenge results.


3. The Vote Processor polls the db for locally verified events to prepare the votes before broadcasting them. It also queries for and saves all the broadcasted votes for this challenge event to check if a 2/3 consensus has been achieved before updating the db with the consensus results.


4. The Tx Submitter polls the db for events that received enough consensus votes and sends a MsgAttest after aggregating the votes and signature. 

# Run locally

## Run MySQL in Docker

```shell
docker run --name gnfd-mysql -p 3306:3306 -e MYSQL_ROOT_PASSWORD=root -d mysql:5.7
```

### Create Schema

Create schema in MySQL client:

```shell
CREATE SCHEMA IF NOT EXISTS `challenger` DEFAULT CHARACTER SET utf8 COLLATE utf8_unicode_ci;
```

### Run Greenfield locally in Greenfield repo

```shell
make build
bash ./deployment/localup/localup.sh all 1 
```

### Run Greenfield challenge_test.go to generate some challenges

### Run following codes int Greenfield challenge_test.go to generate validator and relayer private keys

```go
// validator key
fmt.Println(common.Bytes2Hex(s.Validator.GetPrivKey().Bytes()))

// relayer bls key
fmt.Println(common.Bytes2Hex(s.Relayer.GetPrivKey().Bytes()))
```

### Update config in config.json for MySQL/validator key/relayer bls key

### Start challenger

```shell
make build
./build/greenfield-challenger --config-type local --config-path ./config/config.json
```

## Config
```
{
  "greenfield_config": {
    "key_type": local private key
    "aws_region": service region that contains the key to import
    "aws_secret_name": service secret name contains the key to import
    "rpc_addrs": [
      "http://0.0.0.0:26750"
    ],
    "grpc_addrs": [
      "localhost:9090"
    ],
    "private_key": validator private key
    "gas_limit": gas limit
    "chain_id_string": deployment env chain id
    "deduplication_interval": interval before the same event can be processed again
    "heartbeat_interval": routine check to see if this service is still alive
  },
  "vote_pool_config": {
    "rpc_addr": "http://127.0.0.1:26750",
    "bls_private_key": relayer key 
  },
  "log_config": {
    "level": "DEBUG",
    "filename": "log.txt",
    "max_file_size_in_mb": file size threshold
    "max_backups_of_log_files": backup count threshold
    "max_age_to_retain_log_files_in_days": backup age threshold
    "use_console_logger": true,
    "use_file_logger": false,
    "compress": false
  },
  "admin_config": {
    "listen_addr": "0.0.0.0:8080"
  },
  "db_config": {
    "dialect": db type
    "db_path": db path
  },
  "alert_config": {
    "interval": interval before next msg can be sent
    "identity": telegram bot msg sender identity
    "telegram_bot_id": telegram bot id
    "telegram_chat_id": telegram bot chat id  
  }
}
```
