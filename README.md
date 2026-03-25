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

# Folder structure
Each service lives in its own folder. Each folder will have its own README.md for usage 
and contributing instructions. The services will be deployed via helm, which will track 
versions when updates are needed. Each subfolder will use semantic versioning to track 
its own version. There is no versioning for the monorepo as a whole.

* pki > Certificate Authority and REST API for trains to register for certificates.
* trains > Simulated trains that receive commands from rabbitMQ and send telemetry to API Gateway.
* rabbit > RabbitMQ server and management UI.
* kafka > Kafka service for telemetry data.

# Ports (dev / staging / prod):
| container | local | staging | prod |
| -------- | -------- | -------- | -------- |
| pki | 8080 | 8100 | 8200 |
| trains | 8080 | 8101 | 8201 |
| rabbit-server | 5671 | 8102 | 8202 |
| rabbit (mgmt) | 15672 | 8103 | 8203 |
| kafka client | 9092 | 8104 | 8204 |
| kafka control | 9093 | 8105 | 8205 |
