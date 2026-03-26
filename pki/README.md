## This container will run a CA engine and REST API for managing PKI for the railway sim

### Container ports:
* 8080 - REST API for issuing new certs / downloading CA cert

### Endpoints:
* /health # GET - shows version and environment
* /issue  # POST - ask for a new certificate, must have X-Auth-Token header
* /ca     # GET - retrieve the CA cert
* /revoke # POST - revoke the certificate (?serial=<cert_serial>)
* /crl    # GET - retrieve Certificate Revokation List

### Contribute:
* Make branch 
* Modify .go files as needed.
* Update version (Changelog below, Dockerfile)
* Test locally
    * ```export PKIISSUETOKEN="dev"```
    * ```export PKIADMINTOKEN="dev"```
    * ```go run *.go"```
    * Test http://localhost:8080/health etc
* ENSURE YOU ARE IN THE *pki* FOLDER
    * Git commit and push branch
    * Merge in github
    * Switch back to main, pull
        * We will not be tagging / releasing since this is a monorepo
    * Build docker image (see command in Dockerfile)
        * test container if necessary (watch for port conflicts)
        * ```docker run -d --restart=always -p 8080:8080 -e PKIISSUETOKEN=dev -e PKIADMINTOKEN=dev pki:<version>```
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
* *Created*: Initial Development (getting PKI running)