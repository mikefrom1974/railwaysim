## BNSF Simulator (sorta)
This project aims to simulate BNSF Railway systems (largely from guesswork; I have no actual 
experience with them) for the purpose of learning how messaging might work between the trains 
and the central data centers.

# Folder structure
Each service lives in its own folder. Each folder will have its own README.md for usage 
and contributing instructions. The services will be deployed via helm, which will track 
versions when updates are needed.

* helm > deployment config for helm package mgr -> kubernetes
* pki > Certificate Authority and REST API for trains to register for certificates.