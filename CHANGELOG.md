# Changelog

## v0.0.7
This release updates the dependencies to adapt to Greenfield v0.2.6 and adds a bugfix to handle events where components cannot be fetched successfully for hash verification.  

Bugfixes:  
* [#87](https://github.com/bnb-chain/greenfield-challenger/pull/87) fix: handle events where components cannot be fetched successfully for hash verification

## v0.0.6-hf.1
This is a hotfix release to update the outdated config file.

Chore:
* [#86](https://github.com/bnb-chain/greenfield-challenger/pull/86) fix: update outdated config file

## v0.0.6  
This release updates the dependencies to adapt to Greenfield v0.2.5 and contains 1 new feature.

Features:  
* [#80](https://github.com/bnb-chain/greenfield-challenger/pull/80) feat: add logging to metrics and waitgroups for goroutines

## v0.0.5
This release updates the dependencies to adapt to Greenfield v0.2.4 and contains 1 new feature.  

Features:  
* [#73](https://github.com/bnb-chain/greenfield-challenger/pull/73) feat: enable using env vars for config

Chores:
* [#76](https://github.com/bnb-chain/greenfield-challenger/pull/76) chore: clean up unused code and update dependencies

CI:  
* [#75](https://github.com/bnb-chain/greenfield-challenger/pull/75) ci: add docker job for tag releasing
* [#74](https://github.com/bnb-chain/greenfield-challenger/pull/74) ci: Create Dockerfile.distroless


## v0.0.5-alpha.1
This release updates the dependencies to adapt to Greenfield v0.2.4-alpha.2

## v0.0.4
This release includes a feature and bugfix. Metrics are added in this release.

Features:
* [#61](https://github.com/bnb-chain/greenfield-challenger/pull/61) feat: add metrics

Bugfixes:
* [#64](https://github.com/bnb-chain/greenfield-challenger/pull/64) fix: attest monitor missing attested challenges

Chores:
* [#63](https://github.com/bnb-chain/greenfield-challenger/pull/63) chore: add issue template


## v0.0.3
This release includes 1 feature and 2 bugfixes.

Features:
* [#58](https://github.com/bnb-chain/greenfield-challenger/pull/58) feat: pass in chainid when calculating event hash

Bugfixes:
* [#56](https://github.com/bnb-chain/greenfield-challenger/pull/56) fix: fix issue with hash verifier skipping challenges
* [#54](https://github.com/bnb-chain/greenfield-challenger/pull/54) fix: remove bucketdeleted status

Documentation
* [#57](https://github.com/bnb-chain/greenfield-challenger/pull/57) docs: update README.md

## v0.0.2
This is a maintenance release that updates the service to adapt to it's updated dependencies and code refactoring.
* [#47](https://github.com/bnb-chain/greenfield-challenger/pull/47) feat: adapt to new go-sdk and greenfield version

## v0.0.2-alpha.1
This is a pre-release. The go-sdk module and it's relevant dependencies are updated to use their pre-release alpha versions.
* [#44](https://github.com/bnb-chain/greenfield-challenger/pull/44) chore: upgrade dependencies    

## v0.0.1
This release launches the Challenger Service.

* [#19](https://github.com/bnb-chain/greenfield-challenger/pull/19) fix: added error logs  
* [#20](https://github.com/bnb-chain/greenfield-challenger/pull/20) fix: fix hash verifier issue
* [#21](https://github.com/bnb-chain/greenfield-challenger/pull/21) refactor: refine some codes
* [#22](https://github.com/bnb-chain/greenfield-challenger/pull/22) refactor: refine codes to skip events and query heartbeat interval from blockchain
* [#23](https://github.com/bnb-chain/greenfield-challenger/pull/23) add lint and gosec workflows
* [#24](https://github.com/bnb-chain/greenfield-challenger/pull/24) doc: add deployment documentation 
* [#25](https://github.com/bnb-chain/greenfield-challenger/pull/25) license: add opensource license 
* [#28](https://github.com/bnb-chain/greenfield-challenger/pull/28) fix: fix lock issue 
* [#29](https://github.com/bnb-chain/greenfield-challenger/pull/29) fix: fix some bugs 
* [#30](https://github.com/bnb-chain/greenfield-challenger/pull/30) fix: change hash verification procedure 
* [#31](https://github.com/bnb-chain/greenfield-challenger/pull/31) fix: add heartbeat interval to hash verifier 
* [#32](https://github.com/bnb-chain/greenfield-challenger/pull/32) fix: return error if 2/3 votes have not been achieved 
* [#34](https://github.com/bnb-chain/greenfield-challenger/pull/34) fix querying cached validators out of range error 
* [#36](https://github.com/bnb-chain/greenfield-challenger/pull/36) feat: added vote collector, refactored broadcast 
* [#38](https://github.com/bnb-chain/greenfield-challenger/pull/38) feat: refactor tx submitter 
* [#39](https://github.com/bnb-chain/greenfield-challenger/pull/39) chore: update cosmos sdk to v0.47 
* [#40](https://github.com/bnb-chain/greenfield-challenger/pull/40) fix: implement fix for issues found during qa testing 
