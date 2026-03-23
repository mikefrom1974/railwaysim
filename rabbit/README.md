## This container will run a rabbitMQ server to act as secure control for the trains

### Exposed ports:
* 5672 - Standard AMQP
* 5671 - AMQP over TLS
* 15672 - Management UI

### Contribute:
* Make branch 
* Modify files as needed.
* Update version (Changelog below, Dockerfile)
* Test locally
    * Don't. This is set up to be run as a container that registers with the PKI
* ENSURE YOU ARE IN THE *rabbit* FOLDER
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

### Changelog (Semantic Versioning):
**v0.1.0**
* *Created*: Initial Development (getting rabbitMW server running)