## This container will run a CA engine and REST API for managing PKI for the railway sim

### Exposed ports:
* 8080 - REST API for issuing new certs / downloading CA cert

### Endpoints:

### Contribute:
* Make branch 
* Modify .go files as needed.
* Update version (Changelog below, Dockerfile, helm staging or prod)
* Test locally
    ```export ISSUETOKEN="dev"```
    ```export ADMINTOKEN="dev"```
    ```go run *.go"```
    Test http://localhost:8080/version etc
* ENSURE YOU ARE IN THE pki FOLDER
    * Git commit and push branch
    * Merge in github
    * Switch back to main, pull, tag with version matching Changelog
        * ```git tag v<version>```
        * ```git push origin v<version>```
        * Make release in github
    * Build docker image (see command in Dockerfile)
        * Test container if needed
        * ```docker run -d --restart=always -p 8080:8080 -e ISSUETOKEN=dev -e ADMINTOKEN=dev pki:<version>```
        * If needed, wipe local containers
            * ```docker ps -a```
            * ```docker stop <container id or name>```
            * ```docker rm <container id or name>```
            * ```docker images```
            * ```docker rmi <image id>```
* ENSURE YOU ARE IN THE helm FOLDER
    * Use helm to upgrade the pods (see helm folder)

### Changelog (Semantic Versioning):
**v0.1.0**
* *Created*: Initial Development (getting PKI running)