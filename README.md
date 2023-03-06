# greenfield-challenger

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