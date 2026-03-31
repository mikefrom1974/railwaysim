## This container will run a lightweight web server for the SPOG.
### Container ports:
* 8080: Web endpoint

### Notes:
* index.html is embedded at compile time so will not exist in the container file system.

### Contribute:
* Make branch 
* Modify files as needed.
* Update version (Changelog below, Dockerfile)
* Test locally
* ENSURE YOU ARE IN THE *spog* FOLDER
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
* *Created*: Initial Development (getting Kafka server running)