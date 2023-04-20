# Greenfield Challenger
Greenfield ensures data integrity by routinely issuing storage providers challenge events to prove that the stored data is not tampered. This service allows end users to monitor the blockchain for challenge events and conduct verification upon downloading the hash pieces belonging to the stored object. 

## Disclaimer
**The software and related documentation are under active development, all subject to potential future change without
notification and not ready for production use. The code and security audit have not been fully completed and not ready
for any bug bounty. We advise you to be careful and experiment on the network at your own risk. Stay safe out there.**


## Main Components
This off-chain application comprises of 6 main goroutines: Monitor, Verifier, Vote Collector, Vote Broadcaster, Vote Collator and Tx Submitter.

1. The Monitor polls the blockchain for new blocks to parse for challenge events and adds them to the local db.


2. The Verifier is charge of verifying the integrity of the stored data. The process involves querying the Storage Provider for the piece hashes and the Blockchain for the original hash. A root hash would be computed using the piece hashes received from the Storage Provider. Both the root hash and original hash would then be compared to check if they are equal before updating the db with the challenge results.


3. The Vote Broadcaster retrieves events that failed the verification process and were found to have mismatched hashes. A vote would be constructed and signed before being broadcasted to the blockchain where other Challenger services would be querying from to collect enough votes for a 2/3 consensus.


4. The Vote Collector polls the blockchain for votes that were broadcasted by other Challenger services and adds them to the local db. Votes will undergo validation before they are stored.  


5. The Vote Collator retrieves events that failed the verification process to calculate an event hash. Every ChallengeId has a unique event hash and it would be used to identify votes that were saved in the local db by the Vote Collector. It will then query and collate the votes for a 2/3 consensus before changing the event status to allow the Tx Submitter to process it.  


6. The Tx Submitter polls the db for events that received enough consensus votes and sends a MsgAttest to the blockchain after aggregating the votes and signature. The blockchain will validate the votes and if the attestation passes. the storage provider will then be slashed for failing to protect the integrity of the data that they were tasked to store.    

## Deployment

### Config
1. Set your private key import method (via file or aws secret), deployment environment and gas limit.
```
  "greenfield_config": {
    "key_type": "local_private_key" or "aws_private_key" depending on whether you are storing the keys on aws or locally in this json file
    "aws_region": set this if you chose "aws_private_key"
    "aws_secret_name": set this if you chose "aws_private_key"
    "aws_bls_secret_name": set this if you chose "aws_private_key"
    "private_key": set this if you chose "local_private_key"
    "bls_private_key": set this if you chose "local_private_key" 
    "rpc_addrs": [
      "http://0.0.0.0:26750"
    ],
    "grpc_addrs": [
      "localhost:9090"
    ],
    "gas_limit": 100 (your tx gas limit)
    "chain_id_string": chain id of the network, e.g., "greenfield_9000-121"  
    "deduplication_interval": 100 (skip processing event if recently processed within X events)
  }
```

2. Set rpc addr for vote pool
```
"vote_pool_config": {
  "rpc_addr": "http://127.0.0.1:26750",
}
```

3. Set your log and backup preferences.
```
"log_config": {
  "level": "DEBUG",
  "filename": "log.txt",
  "max_file_size_in_mb": 100 (file size threshold)  
  "max_backups_of_log_files": 2 (backup count threshold)
  "max_age_to_retain_log_files_in_days": 10 (backup age threshold)
  "use_console_logger": true,
  "use_file_logger": false,
  "compress": false
}
```

4. Config your database settings.
```
"db_config": {
  "dialect": "mysql",
  "db_path": "tcp(127.0.0.1:3306)/challenger?charset=utf8&parseTime=True&loc=Local"
  "key_type": "local_private_key" or "aws_private_key" depending on whether you are storing the keys on aws or locally in this json file
  "aws_region": set this if you chose "aws_private_key"
  "aws_secret_name": set this if you chose "aws_private_key"
  "username": set this if you chose "local_private_key"
  "password": set this if you chose "local_private_key"
  "max_idle_conns": 20, (set depending on your db performance)
  "max_open_conns": 40, (set depending on your db performance)
  "debug_mode": false  
}
```

5. Set alert config to send a telegram message when the application exceeds the max retries for certain operations.

```
"alert_config": {
  "identity": your_bot_identity
  "telegram_bot_id": your_bot_id
  "telegram_chat_id": your_chat_id  
}
```


## Run Locally

### Run MySQL in Docker

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
# please refer to greenfield repo for more information
make build
bash ./deployment/localup/localup.sh all 1 7 
```

### Get validator bls key and challenger key

You can use the following approach if you do not know how. 

Run following codes in Greenfield e2e tests to get validator and challenger private keys
```go
// please refer to greenfield repo for more information
// challenger key
fmt.Println(common.Bytes2Hex(s.Challenger.GetPrivKey().Bytes()))

// bls key
fmt.Println(common.Bytes2Hex(s.Validator.GetBlsPrivKey().Bytes()))
```

### Update config.json for keys, and MySQL

### Start challenger

```shell
make build
./build/greenfield-challenger --config-type local --config-path ./config/config.json
```

## Contribute

Thank you for considering to help out with the source code! We welcome contributions
from anyone, and are grateful for even the smallest of fixes!

Please fork, fix, commit and send a pull request
for the maintainers to review and merge into the main code base if you would like to. 

Please make sure your contributions adhere to our coding guidelines:

* Code must adhere to the official Go [formatting](https://golang.org/doc/effective_go.html#formatting)
  guidelines (i.e. uses [gofmt](https://golang.org/cmd/gofmt/)).
* Code must be documented adhering to the official Go [commentary](https://golang.org/doc/effective_go.html#commentary)
  guidelines.
* Pull requests need to be based on and opened against the `master` branch.
* Commit messages should be prefixed with the package(s) they modify.


## License
The repo is licensed under the
[GNU Affero General Public License v3.0](https://www.gnu.org/licenses/agpl-3.0.en.html), also
included in our repository in the `COPYING` file.