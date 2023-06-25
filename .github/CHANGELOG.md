# Changelog

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
