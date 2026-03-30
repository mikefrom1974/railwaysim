## This container will run a Redis server to store recent train telemetry

### Container ports:
* 6379: Redis port

### Contribute:
* Make branch 
* Modify files as needed.
* Update version (Changelog below, Dockerfile)
* Test locally
* ENSURE YOU ARE IN THE *redis* FOLDER
    * Git commit and push branch
    * Merge in github
    * Switch back to main, pull
        * We will not be tagging / releasing since this is a monorepo
    * Build docker image (see command in Dockerfile)
        * If needed, wipe unneeded / conflicting containers
            * ```docker ps -a```
            * ```docker stop <container id or name>```
            * ```docker rm <container id or name>```
            * ```docker images```
            * ```docker rmi <image id>```
    * Once you're ready to push the new container version into production:
        * update docker-compose.yml in root folder with the new version (try in staging first!)
        * then ENSURE YOU'RE IN THE ROOT FOLDER
        * ```source ./secrets.sh```
        * ```docker-compose up -d```

### Sample CLI commands
* ensure ACL is active (should get NOAUTH error)
```docker exec -it redis-server-staging redis-cli PING```
* test with user and pass
```
docker exec -it redis-server-staging redis-cli
# Once inside the prompt:
AUTH <REDIS_USER> <REDIS_PASS> # should get OK
HSET train:4001 status "cruising" speed 45.0 # should get (integer) 1 or 2
SET invalid_key status "invalid" # should get NOPERM
FLUSHALL # should get NOPERM
KEYS train:* # see current state of trains
HGETALL train:4001 # see current telemetry for a given train
```

### Changelog (Semantic Versioning):
**v0.1.0**
* *Created*: Initial Development (getting Kafka server running)