# Greenfield Challenger
Greenfield ensures data integrity by routinely issuing storage providers challenge events to prove that the stored data is not tampered. This service allows end users to monitor the blockchain for challenge events and conduct verification upon downloading the hash pieces belonging to the stored object. 

## Disclaimer
**The software and related documentation are under active development, all subject to potential future change without
notification and not ready for production use. The code and security audit have not been fully completed and not ready
for any bug bounty. We advise you to be careful and experiment on the network at your own risk. Stay safe out there.**


## Main Components
This off-chain application mainly comprises of 4 working parts: Monitor, Verifier, Vote Processor and Tx Submitter. 
1. The Monitor polls the blockchain for new challenge events and adds them to the local db for further processing.


2. The Verifier would then retrieve the event from the db before querying the Storage Provider for the piece hashes and the Blockchain for the original hash. A root hash would be computed using the piece hashes received from the Storage Provider. Both the root hash and original hash would then be compared to check if they are equal before updating the db with the challenge results.


3. The Vote Processor polls the db for locally verified events to prepare the votes before broadcasting them. It also queries for and saves all the broadcasted votes for the challenge events to check if a 2/3 consensus has been achieved before updating the db with the consensus results.


4. The Tx Submitter polls the db for events that received enough consensus votes and sends a MsgAttest after aggregating the votes and signature. 

## Deployment

### Config
1. Set your private key import method (via file or aws secret), deployment environment and gas limit.
```
  "greenfield_config": {
    "key_type": "local_private_key" or "aws_private_key" depending on your choice of import 
    "aws_region": set this if you choose to import using aws
    "aws_secret_name": set this if you choose to import using aws
    "rpc_addrs": [
      "http://0.0.0.0:26750"
    ],
    "grpc_addrs": [
      "localhost:9090"
    ],
    "private_key": challenger_private_key
    "gas_limit": 100 (your tx gas limit)
    "chain_id_string": chain id of the network, e.g., "greenfield_9000-121"  
    "deduplication_interval": 100 (skip processing event if recently processed within X events)
  }
```

2. Set bls key of your validator.
```
"vote_pool_config": {
  "rpc_addr": "http://127.0.0.1:26750",
  "bls_private_key": relayer_private_key 
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
  "db_path": "root:root@tcp(127.0.0.1:3306)/challenger?charset=utf8&parseTime=True&loc=Local"
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