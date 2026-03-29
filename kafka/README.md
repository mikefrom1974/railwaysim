## This container will run a Kafka server to act as pub/sub between the API Gateway and services.

### Container ports:
* 9092: Broker / Client Communication
* 9093: Controller Communication

### Contribute:
* Make branch 
* Modify files as needed.
* Update version (Changelog below, Dockerfile)
* Test locally
    * Don't. This is set up to be run as a container that registers with the PKI
* ENSURE YOU ARE IN THE *kafka* FOLDER
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
* Set SSL Properties (for secured kafka):
```bash
cat <<EOF > /tmp/client-ssl.properties
security.protocol=SSL
ssl.truststore.type=PEM
ssl.truststore.location=/etc/kafka/certs/ca.pem
# THE NUCLEAR OPTION: Disables the hostname/identity check
ssl.endpoint.identification.algorithm=
EOF
```
* List topics:
```bash
/opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --command-config /tmp/client-ssl.properties \
  --list
```
* Consume topic (add --partition X if needed):
```bash
/opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic train-telemetry \
  --from-beginning \
  --consumer.config /tmp/client-ssl.properties
```
* See message counts (offsets) per topic:
```bash
/opt/kafka/bin/kafka-get-offsets.sh \
  --bootstrap-server localhost:9092 \
  --topic train-telemetry \
  --command-config /tmp/client-ssl.properties
```
* Listen for traffic on Kafka port 9092:
```bash
# from docker host
docker ps | grep kafka-server-staging # get container id
docker run --rm -it \
  --net=container:46f34da27d12 \
  nicolaka/netshoot \
  tcpdump -i eth0 -vv -n 'port 9092'
```

### Changelog (Semantic Versioning):
**v0.1.0**
* *Created*: Initial Development (getting Kafka server running)