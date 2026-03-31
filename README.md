## Railway Simulator (sorta)
This project aims to simulate Railway systems (largely from guesswork; I have no actual 
experience with them) for the purpose of learning how messaging might work between the trains 
and the central data centers.

# Secrets management
This project uses OpenBao for its similarity to HashiCorp Vault. Ensure you have it installed 
and run ```source ./secrets.sh``` to pull secrets into env vars before deploying.

# Deploy
Ensure you are in the root folder.
```docker compose up -d```
(Staging and Prod are labeled as profiles so you can start / stop them individually)
```docker compose --profile staging up -d```

# Important Endpoints:
* http://localhost:8112/ - train controller SPOG
* http://localhost:8113/ - prometheus

# Folder structure
Each service lives in its own folder. Each folder will have its own README.md for usage 
and contributing instructions. The services will be deployed via helm, which will track 
versions when updates are needed. Each subfolder will use semantic versioning to track 
its own version. There is no versioning for the monorepo as a whole.

* pki > Certificate Authority and REST API for trains to register for certificates.
* trains > Simulated trains that receive commands from rabbitMQ and send telemetry to API Gateway.
* rabbit > RabbitMQ server and management UI.
* kafka > Kafka service for telemetry data.
* kong > Kong API Gateway
* ingester > temporary go-between (kong>kafka) as the community kong has a kafka client version issue
* exporter > pulls from kafka and acts as both redis sink and prometheus scrape point
* redis > stores last-telemetry for each train
* bff > REST API that lets SPOG talk easily to redis and rabbit
* spog > React UI to view and control trains
* prometheus > TSDB; scrapes exporter /metrics and feeds Grafana.

# Ports (dev / staging / prod):
| container | local | staging | prod |
| -------- | -------- | -------- | -------- |
| pki | 8080 | 8100 | 8200 |
| trains | 8080 | 8101 | 8201 |
| rabbit-server | 5671 | 8102 | 8202 |
| rabbit (mgmt) | 15672 | 8103 | 8203 |
| kafka client | 9092 | 8104 | 8204 |
| kafka control | 9093 | 8105 | 8205 |
| kong-server | 8001 | 8106 | 8206 |
| kong admin | 8443 | 8107 | 8207 |
| ingester | 8080 | 8108 | 8208 |
| redis | 6379 | 8109 | 8209 |
| exporter | 8080 | 8110 | 8210 |
| bff | 8080 | 8111 | 8211 |
| spog | 8080 | 8112 | 8212 |
| prometheus | 9090 | 8113 | 8213 |